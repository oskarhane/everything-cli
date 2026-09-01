package team

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/output"
	"github.com/oskarhane/google-cli/internal/providers/linear/service"
)

// newListCmd returns `linear team list`: every team in the workspace.
func newListCmd(cfg *app.Config, newSvc service.Dialer[service.TeamService]) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List Linear teams",
		Example: `# List teams as JSON
google-cli linear team list --format json

# List teams as a table; key is the issue-identifier prefix
google-cli linear team list --format table`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			teams, err := svc.ListTeams(cmd.Context())
			if err != nil {
				return err
			}
			rows := make([]map[string]any, 0, len(teams))
			for _, t := range teams {
				rows = append(rows, map[string]any{"id": t.ID, "name": t.Name, "key": t.Key})
			}
			output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format),
				[]string{"id", "name", "key"}, rows, rows)
			return nil
		},
	}
}
