package state

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
)

// newListCmd returns `linear state list`: the workflow states of one team,
// ordered by position.
func newListCmd(cfg *app.Config, newSvc service.Dialer[service.StateService]) *cobra.Command {
	var teamID string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List a Linear team's workflow states",
		Example: `# List a team's states as JSON
everything-cli linear state list --team 9c1e2f3a-... --format json

# List a team's states as a table
everything-cli linear state list --team 9c1e2f3a-... --format table`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			states, err := svc.ListStates(cmd.Context(), teamID)
			if err != nil {
				return err
			}
			rows := make([]map[string]any, 0, len(states))
			for _, s := range states {
				rows = append(rows, map[string]any{
					"id":       s.ID,
					"name":     s.Name,
					"type":     s.Type,
					"position": s.Position,
				})
			}
			output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format),
				[]string{"id", "name", "type", "position"}, rows, rows)
			return nil
		},
	}
	cmd.Flags().StringVar(&teamID, "team", "", "Team ID whose states to list (required)")
	_ = cmd.MarkFlagRequired("team")
	return cmd
}
