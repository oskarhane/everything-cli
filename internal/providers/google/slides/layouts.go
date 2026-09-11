package slides

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
)

// layoutsFields is the field order for layout output; the same names are the
// snake_case JSON and TOON keys. go-pretty's StyleLight upper-cases the
// headers when rendering.
var layoutsFields = []string{"object_id", "name"}

// newLayoutsCmd returns `slides layouts`: the presentation's layouts, each
// with the object ID `add --layout` accepts and the display name a human
// recognizes. This is the discovery verb for the add leaf.
func newLayoutsCmd(cfg *app.Config, newSvc service.Dialer[service.SlideService]) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "layouts <presentation-id>",
		Short: "List a presentation's layouts (name and object ID)",
		Example: `# List every layout as JSON
everything-cli google slides layouts 1AbCpresentationID --format json

# Show the same layouts as a table
everything-cli google slides layouts 1AbCpresentationID --format table

# Find the object ID to pass to add --layout
everything-cli google slides layouts 1AbCpresentationID --format json | jq '.[] | select(.name=="Title and Body").object_id'`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			layouts, err := svc.ListSlideLayouts(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			rows := make([]map[string]any, 0, len(layouts))
			for _, layout := range layouts {
				rows = append(rows, map[string]any{
					"object_id": layout.ObjectID,
					"name":      layout.Name,
				})
			}
			output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format), layoutsFields, rows, rows)
			return nil
		},
	}
	return cmd
}
