package slides

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
)

// addFields is the field order for the created-slide echo; the same name is
// the snake_case JSON and TOON key. go-pretty's StyleLight upper-cases the
// header when rendering.
var addFields = []string{"slide_id"}

// newAddCmd returns `slides add`: create a slide from a layout and print the
// new slide's object ID. --layout takes either the layout's display name (as
// listed by `slides layouts`) or its object ID; the name is resolved against
// a live Presentations.Get so the wire always carries the object ID.
// --index places the slide at a zero-based position; omitted, it is appended.
func newAddCmd(cfg *app.Config, newSvc service.Dialer[service.SlideService]) *cobra.Command {
	var (
		layout string
		index  int64
	)
	cmd := &cobra.Command{
		Use:   "add <presentation-id>",
		Short: "Add a slide from a layout",
		Example: `# Append a slide using the layout named "Title and Body"
everything-cli google slides add 1AbCpresentationID --layout "Title and Body"

# Insert a title-only slide at the very front (zero-based index 0)
everything-cli google slides add 1AbCpresentationID --layout TITLE_ONLY --index 0

# Pass the layout's object ID directly (from slides layouts)
everything-cli google slides add 1AbCpresentationID --layout g1f2d3c4b5`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if layout == "" {
				return fmt.Errorf("--layout is required: give a layout name or object ID (run `everything-cli google slides layouts %s`)", args[0])
			}
			if cmd.Flags().Changed("index") && index < 0 {
				return fmt.Errorf("invalid --index %d: must be non-negative", index)
			}
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			layouts, err := svc.ListSlideLayouts(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			layoutID, err := resolveLayout(layouts, layout)
			if err != nil {
				return err
			}
			var at *int64
			if cmd.Flags().Changed("index") {
				at = &index
			}
			slideID, err := svc.CreateSlide(cmd.Context(), args[0], layoutID, at)
			if err != nil {
				return err
			}
			output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format), addFields,
				map[string]any{"slide_id": slideID}, []map[string]any{{"slide_id": slideID}})
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&layout, "layout", "", "Layout to build the slide from, by display name or object ID (required)")
	f.Int64Var(&index, "index", 0, "Zero-based position to insert the slide at (default: the end)")
	return cmd
}

// resolveLayout maps a --layout key to a layout object ID: an exact object
// ID wins first, then an exact (case-insensitive) display-name match, so a
// name never shadows a same-string ID. The error names the offending key.
func resolveLayout(layouts []service.SlideLayout, key string) (string, error) {
	for _, layout := range layouts {
		if layout.ObjectID == key {
			return layout.ObjectID, nil
		}
	}
	for _, layout := range layouts {
		if strings.EqualFold(layout.Name, key) {
			return layout.ObjectID, nil
		}
	}
	return "", fmt.Errorf("no layout named or with object ID %q in the presentation (run `everything-cli google slides layouts <presentation-id>` to list them)", key)
}
