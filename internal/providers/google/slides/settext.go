package slides

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
	"github.com/oskarhane/everything-cli/internal/providers/google/textflag"
)

// newSetTextCmd returns `slides set-text`: write text into one placeholder
// shape on one slide. --slide takes a 1-based slide number or a slide object
// ID; --placeholder-idx names the placeholder's index on that slide. Both are
// resolved against a live Presentations.Get to the shape's object ID, which
// the InsertTextRequest at index 0 targets. The text comes from --text or a
// file via --text-file, exactly one of the two.
func newSetTextCmd(cfg *app.Config, newSvc service.Dialer[service.SlideService]) *cobra.Command {
	var (
		text           string
		textFile       string
		slide          string
		placeholderIdx int64
	)
	cmd := &cobra.Command{
		Use:   "set-text <presentation-id>",
		Short: "Write text into a placeholder shape on a slide",
		Example: `# Set the title (placeholder index 0) on the second slide
everything-cli google slides set-text 1AbCpresentationID --slide 2 --placeholder-idx 0 --text "Q4 Roadmap"

# Write the same text from a file, addressing the slide by object ID
everything-cli google slides set-text 1AbCpresentationID --slide g1f2d3c4b5 --placeholder-idx 1 --text-file body.txt`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if slide == "" {
				return fmt.Errorf("--slide is required: give a 1-based slide number or a slide object ID")
			}
			if !cmd.Flags().Changed("placeholder-idx") {
				return fmt.Errorf("--placeholder-idx is required: give the placeholder's index on the slide (run `everything-cli google slides get %s` to inspect the slide)", args[0])
			}
			body, err := textflag.Resolve(cfg.Fs, text, textFile, "set")
			if err != nil {
				return err
			}
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			placeholders, err := svc.ListSlidePlaceholders(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			shapeID, err := resolvePlaceholder(placeholders, slide, placeholderIdx)
			if err != nil {
				return err
			}
			if err := svc.InsertSlideText(cmd.Context(), args[0], shapeID, body); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Set text on shape %s in presentation %s\n", shapeID, args[0]); err != nil {
				return err
			}
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&text, "text", "", "Text to write (replaces the placeholder's content)")
	f.StringVar(&textFile, "text-file", "", "Read the text to write from this file instead of --text")
	f.StringVar(&slide, "slide", "", "Slide to write to, by 1-based number or object ID (required)")
	f.Int64Var(&placeholderIdx, "placeholder-idx", 0, "Index of the placeholder shape on the slide (required)")
	return cmd
}

// resolvePlaceholder maps a slide key plus placeholder index to the target
// shape's object ID. A numeric key is a 1-based slide position; anything else
// is matched against slide object IDs. The error names whichever value is
// wrong: the slide (out of range or unknown) or the placeholder index.
func resolvePlaceholder(placeholders []service.SlidePlaceholder, slideKey string, idx int64) (string, error) {
	if n, err := strconv.Atoi(slideKey); err == nil {
		if n < 1 {
			return "", fmt.Errorf("slide %d is out of range: slides are numbered from 1", n)
		}
		max := 0
		for _, p := range placeholders {
			if p.SlideNumber > max {
				max = p.SlideNumber
			}
		}
		if n > max {
			return "", fmt.Errorf("slide %d is out of range: the presentation has %d slide(s) with placeholders", n, max)
		}
		for _, p := range placeholders {
			if p.SlideNumber == n && p.PlaceholderIndex == idx {
				return p.ShapeID, nil
			}
		}
		return "", fmt.Errorf("slide %d has no placeholder with index %d", n, idx)
	}
	foundSlide := false
	for _, p := range placeholders {
		if p.SlideID != slideKey {
			continue
		}
		foundSlide = true
		if p.PlaceholderIndex == idx {
			return p.ShapeID, nil
		}
	}
	if foundSlide {
		return "", fmt.Errorf("slide %s has no placeholder with index %d", slideKey, idx)
	}
	return "", fmt.Errorf("no slide with object ID %q in the presentation", slideKey)
}
