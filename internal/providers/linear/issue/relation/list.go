package relation

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
)

// newListCmd returns `linear issue relation list`: every relation touching
// --issue, outgoing and incoming, rendered from that issue's perspective.
func newListCmd(cfg *app.Config, newSvc service.Dialer[service.RelationService]) *cobra.Command {
	var issue string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List an issue's relations in both directions",
		Example: `# List every relation touching BLA-123 as JSON
everything-cli linear issue relation list --issue BLA-123 --format json

# The same list as a table
everything-cli linear issue relation list --issue BLA-123 --format table`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			relations, err := svc.ListRelations(cmd.Context(), issue)
			if err != nil {
				return err
			}
			printRelationList(cmd, cfg, relations)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&issue, "issue", "", "Issue UUID or identifier to list relations for (required)")
	_ = cmd.MarkFlagRequired("issue")
	return cmd
}

// printRelationList renders a relation list under the one-row-vs-array
// output convention; the view is flat, so one row shape serves JSON, TOON,
// and table.
func printRelationList(cmd *cobra.Command, cfg *app.Config, relations []service.Relation) {
	rows := make([]map[string]any, 0, len(relations))
	for i := range relations {
		rows = append(rows, row(toView(&relations[i])))
	}
	output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format), Fields, rows, rows)
}
