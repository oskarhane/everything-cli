package project

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
)

// newListCmd returns `linear project list`: every project in the workspace.
func newListCmd(cfg *app.Config, newSvc service.Dialer[service.ProjectService]) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List Linear projects",
		Example: `# List projects as JSON
everything-cli linear project list --format json

# List projects as a table
everything-cli linear project list --format table`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			projects, err := svc.ListProjects(cmd.Context())
			if err != nil {
				return err
			}
			rows := make([]map[string]any, 0, len(projects))
			for _, p := range projects {
				rows = append(rows, map[string]any{
					"id": p.ID, "name": p.Name, "description": p.Description, "state": p.State,
				})
			}
			output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format),
				[]string{"id", "name", "description", "state"}, rows, rows)
			return nil
		},
	}
}
