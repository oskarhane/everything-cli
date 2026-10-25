package image

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
)

// newListCmd returns `linear issue image list`: every markdown image
// reference in one issue's description — plus every comment body when
// --comments is set — deduplicated by URL, rendered as one row per image.
func newListCmd(cfg *app.Config, d Dialers) *cobra.Command {
	var withComments bool
	cmd := &cobra.Command{
		Use:   "list <id>",
		Short: "List images referenced by a Linear issue",
		Example: `# List the images in an issue's description as JSON
everything-cli linear issue image list BLA-123 --format json

# Include images referenced from comments (all pages)
everything-cli linear issue image list BLA-123 --comments`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			refs, err := discoverRefs(cmd.Context(), d, args[0], withComments)
			if err != nil {
				return err
			}
			printImageList(cmd, cfg, refs)
			return nil
		},
	}
	cmd.Flags().BoolVar(&withComments, "comments", false, "Also scan comment bodies")
	return cmd
}

// imageListFields are the table columns of image list.
var imageListFields = []string{"filename", "source", "url"}

// printImageList renders one row per image ref under the one-row-vs-array
// output convention. The filename is the alt text sanitized to a bare
// basename — the name `image download` would write, before its
// response-driven extension fixing.
func printImageList(cmd *cobra.Command, cfg *app.Config, refs []imageRef) {
	rows := make([]map[string]any, 0, len(refs))
	for _, ref := range refs {
		rows = append(rows, map[string]any{
			"url":      ref.URL,
			"filename": sanitizeFilename(ref.Alt),
			"source":   ref.Source,
		})
	}
	output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format), imageListFields, rows, rows)
}
