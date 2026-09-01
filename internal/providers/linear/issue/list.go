package issue

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
)

// newListCmd returns `linear issue list`: issues, most recently updated
// first, optionally scoped to one team.
func newListCmd(cfg *app.Config, newSvc service.Dialer[service.IssueService]) *cobra.Command {
	var teamID string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Linear issues",
		Example: `# List workspace issues as JSON
everything-cli linear issue list --format json

# List one team's issues as a table
everything-cli linear issue list --team 9c1e2f3a-... --format table`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			issues, err := svc.ListIssues(cmd.Context(), teamID)
			if err != nil {
				return err
			}
			printIssueList(cmd, cfg, issues)
			return nil
		},
	}
	cmd.Flags().StringVar(&teamID, "team", "", "Team ID to scope the listing (empty = workspace-wide)")
	return cmd
}
