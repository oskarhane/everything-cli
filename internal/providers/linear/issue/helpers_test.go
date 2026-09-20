package issue

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

// TestMain neutralizes format auto-detection so the host's harness env and
// TTY cannot flip output expectations.
func TestMain(m *testing.M) {
	output.IsAgent = func() bool { return false }
	output.StdoutIsTerminal = func() bool { return false }
	os.Exit(m.Run())
}

// fakeService is the hermetic service.IssueService double: it serves seeded
// issues and records every call for assertions. Its ListComments and
// CreateComment stubs also satisfy service.CommentService, so the comments
// leaf — which dials a CommentService — can reuse it.
type fakeService struct {
	issues   []service.Issue
	comments []service.Comment
	err      error // when set, every call fails

	listFilter      service.IssueFilter
	commentsIssueID string
	gotID           string
	created         service.CreateIssueInput
	updatedID       string
	updated         service.UpdateIssueInput
}

func (f *fakeService) ListIssues(_ context.Context, filter service.IssueFilter) ([]service.Issue, error) {
	f.listFilter = filter
	if f.err != nil {
		return nil, f.err
	}
	return f.issues, nil
}

func (f *fakeService) ListComments(_ context.Context, issueID string) ([]service.Comment, error) {
	f.commentsIssueID = issueID
	if f.err != nil {
		return nil, f.err
	}
	return f.comments, nil
}

// CreateComment is a stub: the issue-package leaves never create comments,
// but the fake must satisfy service.CommentService for the comments leaf.
func (f *fakeService) CreateComment(_ context.Context, _ string, _ service.CreateCommentInput) (*service.Comment, error) {
	if f.err != nil {
		return nil, f.err
	}
	return nil, nil
}

func (f *fakeService) GetIssue(_ context.Context, id string) (*service.Issue, error) {
	f.gotID = id
	if f.err != nil {
		return nil, f.err
	}
	for i := range f.issues {
		if f.issues[i].ID == id || f.issues[i].Identifier == id {
			return &f.issues[i], nil
		}
	}
	return nil, fmt.Errorf("issue %q not found", id)
}

func (f *fakeService) CreateIssue(_ context.Context, in service.CreateIssueInput) (*service.Issue, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.created = in
	created := seedIssue()
	created.Identifier = "ENG-4"
	created.Title = in.Title
	return &created, nil
}

func (f *fakeService) UpdateIssue(_ context.Context, id string, in service.UpdateIssueInput) (*service.Issue, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.updatedID, f.updated = id, in
	updated := seedIssue()
	if in.Title != "" {
		updated.Title = in.Title
	}
	return &updated, nil
}

// fakeNewSvc hands out svc so leaves run hermetically with no network and
// no real account store.
func fakeNewSvc(svc *fakeService) service.Dialer[service.IssueService] {
	return func(context.Context) (service.IssueService, error) { return svc, nil }
}

// seedIssue returns one issue carrying every rendered field.
func seedIssue() service.Issue {
	return service.Issue{
		ID:          "issue_1",
		Identifier:  "ENG-1",
		Title:       "Fix login redirect",
		Description: "Users land on / after logout",
		State:       &service.StateRef{ID: "state_1", Name: "In Progress", Type: "started"},
		Assignee:    &service.NamedRef{ID: "user_1", Name: "Ada"},
		Creator:     &service.NamedRef{ID: "user_2", Name: "Grace"},
		Team:        &service.Team{ID: "team_1", Name: "Engineering", Key: "ENG"},
		URL:         "https://linear.app/x/issue/ENG-1",
		CreatedAt:   "2026-08-01T10:00:00.000Z",
		UpdatedAt:   "2026-08-02T10:00:00.000Z",
		StartedAt:   "2026-08-01T11:00:00.000Z",
	}
}

// newLeafCmd builds a leaf against a fake service, ready to execute.
func newLeafCmd(build func(*app.Config, service.Dialer[service.IssueService]) *cobra.Command, svc *fakeService, format string) *cobra.Command {
	return build(cmdtest.NewTestConfig(format), fakeNewSvc(svc))
}

// fakeNewCommentSvc hands out svc as a service.CommentService so the
// comments leaf runs hermetically.
func fakeNewCommentSvc(svc *fakeService) service.Dialer[service.CommentService] {
	return func(context.Context) (service.CommentService, error) { return svc, nil }
}

// newCommentLeafCmd builds the comments leaf against a fake service, ready
// to execute.
func newCommentLeafCmd(svc *fakeService, format string) *cobra.Command {
	return newCommentsCmd(cmdtest.NewTestConfig(format), fakeNewCommentSvc(svc))
}

// serverSvc returns a dialer bound to the real service against srv — the
// leaf exercises the actual GraphQL plumbing hermetically.
func serverSvc(srv *httptest.Server) service.Dialer[service.IssueService] {
	return func(context.Context) (service.IssueService, error) {
		return service.NewForEndpoint(&http.Client{}, srv.URL), nil
	}
}
