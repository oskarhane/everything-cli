package comment

import (
	"context"
	"os"
	"testing"

	"github.com/spf13/cobra"

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

// fakeService is the hermetic service.CommentService double: it returns the
// seeded created comment and records the issue id and input it receives.
type fakeService struct {
	created *service.Comment
	err     error // when set, CreateComment fails

	createIssueID string
	createInput   service.CreateCommentInput
}

func (f *fakeService) ListComments(_ context.Context, _ string) ([]service.Comment, error) {
	return nil, nil
}

func (f *fakeService) CreateComment(_ context.Context, issueID string, in service.CreateCommentInput) (*service.Comment, error) {
	f.createIssueID, f.createInput = issueID, in
	if f.err != nil {
		return nil, f.err
	}
	return f.created, nil
}

// fakeNewSvc hands out svc so the leaf runs hermetically with no network and
// no real account store.
func fakeNewSvc(svc *fakeService) service.Dialer[service.CommentService] {
	return func(context.Context) (service.CommentService, error) { return svc, nil }
}

// createCmd builds the create leaf against a fake service, ready to execute.
func createCmd(svc *fakeService, format string) *cobra.Command {
	return newCreateCmd(cmdtest.NewTestConfig(format), fakeNewSvc(svc))
}

// seedComment returns a created comment carrying every rendered field.
func seedComment() *service.Comment {
	return &service.Comment{
		ID:        "comment_1",
		Body:      "Looks good to me",
		CreatedAt: "2026-08-01T10:00:00.000Z",
		UpdatedAt: "2026-08-01T10:00:00.000Z",
		User:      &service.NamedRef{ID: "user_1", Name: "Ada"},
	}
}
