package label

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
)

// newListCmd returns `linear label list`: the issue labels of one team.
func newListCmd(cfg *app.Config, newSvc service.Dialer[service.LabelService]) *cobra.Command {
	var teamID string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List a Linear team's issue labels",
		Example: `# List a team's labels as JSON
everything-cli linear label list --team 9c1e2f3a-... --format json

# List a team's labels as a table
everything-cli linear label list --team 9c1e2f3a-... --format table`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			labels, err := svc.ListTeamLabels(cmd.Context(), teamID)
			if err != nil {
				return err
			}
			rows := make([]map[string]any, 0, len(labels))
			for _, l := range labels {
				rows = append(rows, map[string]any{
					"id":    l.ID,
					"name":  l.Name,
					"color": l.Color,
				})
			}
			output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format),
				[]string{"id", "name", "color"}, rows, rows)
			return nil
		},
	}
	cmd.Flags().StringVar(&teamID, "team", "", "Team ID whose labels to list (required)")
	_ = cmd.MarkFlagRequired("team")
	return cmd
}
