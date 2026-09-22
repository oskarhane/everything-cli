package linear

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/auth"
	"github.com/oskarhane/everything-cli/internal/config"
	"github.com/oskarhane/everything-cli/internal/providers/linear/account"
	"github.com/oskarhane/everything-cli/internal/providers/linear/issue"
	"github.com/oskarhane/everything-cli/internal/providers/linear/project"
	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
	"github.com/oskarhane/everything-cli/internal/providers/linear/state"
	"github.com/oskarhane/everything-cli/internal/providers/linear/team"
)

// newLinearCmd returns the `linear` parent command with its subtrees
// attached. Each subtree lives in its own package; every leaf lives in its
// own file with one AddCommand line per leaf. The concrete service
// implements every linear interface; dialAs narrows the shared seam to
// each subtree's own surface.
func newLinearCmd(cfg *app.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   ID,
		Short: "Interact with Linear from the command line",
	}
	cmd.AddCommand(issue.NewCmd(cfg, issue.Dialers{
		Issue:      dialAs[service.IssueService](cfg),
		Viewer:     dialAs[service.ViewerService](cfg),
		Comment:    dialAs[service.CommentService](cfg),
		Attachment: dialAs[service.AttachmentService](cfg),
		State:      dialAs[service.StateService](cfg),
		Label:      dialAs[service.LabelService](cfg),
		Cycle:      dialAs[service.CycleService](cfg),
		Milestone:  dialAs[service.MilestoneService](cfg),
	}))
	cmd.AddCommand(team.NewCmd(cfg, dialAs[service.TeamService](cfg)))
	cmd.AddCommand(project.NewCmd(cfg, dialAs[service.ProjectService](cfg)))
	cmd.AddCommand(state.NewCmd(cfg, dialAs[service.StateService](cfg)))
	cmd.AddCommand(account.NewCmd(cfg, ID, newAccountStrategy, dialAs[service.ViewerService](cfg)))
	return cmd
}

// newAccountStrategy builds the add-path composite strategy on the store
// account add resolved for the invocation, so the strategy is fully
// constructed: a Client call on it can never dereference a nil store.
func newAccountStrategy(store *config.Store) auth.Strategy { return newStrategy(store) }
