package slides

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
)

// getFields is the field order for slide text output; the same names are the
// snake_case JSON and TOON keys. go-pretty's StyleLight upper-cases the
// headers when rendering.
var getFields = []string{"slide", "shape_id", "text"}

// newGetCmd returns `slides get`: one row per text-bearing shape, in slide
// order. --slide narrows the view to one slide's shapes (1-based, matching
// the slide column).
func newGetCmd(cfg *app.Config, newSvc service.Dialer[service.SlideService]) *cobra.Command {
	var slide int
	cmd := &cobra.Command{
		Use:   "get <presentation-id>",
		Short: "Show the text on every slide of a presentation",
		Example: `# List every text-bearing shape as JSON
everything-cli google slides get 1AbCpresentationID --format json

# Only the shapes on slide 3
everything-cli google slides get 1AbCpresentationID --slide 3

# Same view as a table
everything-cli google slides get 1AbCpresentationID --format table`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			shapes, err := svc.GetSlideText(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if slide > 0 {
				shapes = filterSlide(shapes, slide)
			}
			rows := make([]map[string]any, 0, len(shapes))
			for _, s := range shapes {
				rows = append(rows, shapeRow(s))
			}
			output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format), getFields, rows, rows)
			return nil
		},
	}
	cmd.Flags().IntVar(&slide, "slide", 0, "Only shapes on this 1-based slide number (0 = all slides)")
	return cmd
}

// filterSlide keeps the shapes on the given slide.
func filterSlide(shapes []service.SlideShape, slide int) []service.SlideShape {
	var kept []service.SlideShape
	for _, s := range shapes {
		if s.SlideNumber == slide {
			kept = append(kept, s)
		}
	}
	return kept
}

// shapeRow maps one shape to its output row.
func shapeRow(s service.SlideShape) map[string]any {
	return map[string]any{
		"slide":    s.SlideNumber,
		"shape_id": s.ShapeID,
		"text":     s.Text,
	}
}
