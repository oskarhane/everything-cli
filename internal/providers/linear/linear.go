package linear

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/auth"
	"github.com/oskarhane/everything-cli/internal/providers/linear/account"
	"github.com/oskarhane/everything-cli/internal/providers/linear/issue"
	"github.com/oskarhane/everything-cli/internal/providers/linear/project"
	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
	"github.com/oskarhane/everything-cli/internal/providers/linear/team"
)

// newLinearCmd returns the `linear` parent command with its subtrees
// attached. Each subtree lives in its own package; every leaf lives in its
// own file with one AddCommand line per leaf. The concrete service
// implements every linear interface; service.As narrows the shared seam to
// each subtree's own surface.
func newLinearCmd(cfg *app.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   ID,
		Short: "Interact with Linear from the command line",
	}
	cmd.AddCommand(issue.NewCmd(cfg, func(ctx context.Context) (service.IssueService, error) {
		return service.As[service.IssueService](dial(ctx, cfg))
	}))
	cmd.AddCommand(team.NewCmd(cfg, func(ctx context.Context) (service.TeamService, error) {
		return service.As[service.TeamService](dial(ctx, cfg))
	}))
	cmd.AddCommand(project.NewCmd(cfg, func(ctx context.Context) (service.ProjectService, error) {
		return service.As[service.ProjectService](dial(ctx, cfg))
	}))
	cmd.AddCommand(account.NewCmd(cfg, ID, func() auth.Strategy { return newStrategy() }))
	return cmd
}
