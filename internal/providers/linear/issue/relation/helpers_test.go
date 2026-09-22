package relation

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

// fakeService is the hermetic service.RelationService double: it returns
// seeded results and records the calls it receives. dialed flips when the
// dialer runs, so tests can assert invalid input never reaches the service.
type fakeService struct {
	relations []service.Relation
	created   *service.Relation
	err       error

	dialed        bool
	createIssueID string
	createRelated string
	createType    string
	listIssueID   string
	deletedID     string
}

func (f *fakeService) CreateRelation(_ context.Context, issueID, relatedIssueID, relType string) (*service.Relation, error) {
	f.createIssueID, f.createRelated, f.createType = issueID, relatedIssueID, relType
	if f.err != nil {
		return nil, f.err
	}
	return f.created, nil
}

func (f *fakeService) ListRelations(_ context.Context, issueID string) ([]service.Relation, error) {
	f.listIssueID = issueID
	if f.err != nil {
		return nil, f.err
	}
	return f.relations, nil
}

func (f *fakeService) DeleteRelation(_ context.Context, relationID string) error {
	f.deletedID = relationID
	return f.err
}

// fakeNewSvc hands out svc so the leaves run hermetically with no network
// and no real account store, and records the dial.
func fakeNewSvc(svc *fakeService) service.Dialer[service.RelationService] {
	return func(context.Context) (service.RelationService, error) {
		svc.dialed = true
		return svc, nil
	}
}

// createCmd builds the create leaf against a fake service, ready to execute.
func createCmd(svc *fakeService, format string) *cobra.Command {
	return newCreateCmd(cmdtest.NewTestConfig(format), fakeNewSvc(svc))
}

// listCmd builds the list leaf against a fake service, ready to execute.
func listCmd(svc *fakeService, format string) *cobra.Command {
	return newListCmd(cmdtest.NewTestConfig(format), fakeNewSvc(svc))
}

// deleteCmd builds the delete leaf against a fake service, ready to execute.
func deleteCmd(svc *fakeService, format string) *cobra.Command {
	return newDeleteCmd(cmdtest.NewTestConfig(format), fakeNewSvc(svc))
}

// seedRelations returns one outgoing blocks relation and one incoming
// blocks relation (blocked-by from ENG-1's perspective) touching ENG-1.
func seedRelations() []service.Relation {
	return []service.Relation{
		{
			ID:        "rel_1",
			Type:      "blocks",
			Direction: service.RelationOutgoing,
			Issue: &service.RelationIssue{
				ID: "issue_1", Identifier: "ENG-1", Title: "First",
			},
			RelatedIssue: &service.RelationIssue{
				ID: "issue_2", Identifier: "ENG-2", Title: "Second",
			},
		},
		{
			ID:        "rel_2",
			Type:      "blocks",
			Direction: service.RelationIncoming,
			Issue: &service.RelationIssue{
				ID: "issue_3", Identifier: "ENG-3", Title: "Third",
			},
			RelatedIssue: &service.RelationIssue{
				ID: "issue_1", Identifier: "ENG-1", Title: "First",
			},
		},
	}
}
