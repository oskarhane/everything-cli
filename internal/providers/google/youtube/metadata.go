package youtube

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
	"github.com/oskarhane/everything-cli/internal/youtube"
)

// metadataFields is the metadata leaf's output field order: table column
// order, and the same snake_case names are the JSON and TOON keys.
// go-pretty's StyleLight upper-cases the headers when rendering.
var metadataFields = []string{
	"video_id",
	"title",
	"channel",
	"channel_id",
	"duration_seconds",
	"view_count",
	"publish_date",
	"upload_date",
	"category",
	"description",
	"available_langs",
}

// newMetadataCmd returns `youtube metadata`: one video's metadata for a
// watch URL or bare video ID. available_langs lists every caption track
// language the video offers — one entry per track, human and ASR
// duplicates included — so agents can discover the transcript --lang
// choices.
func newMetadataCmd(cfg *app.Config, client youtube.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "metadata <url-or-id>",
		Short: "Show video metadata and available caption languages",
		Example: `# Show metadata for a watch URL as JSON
everything-cli google youtube metadata "https://www.youtube.com/watch?v=dQw4w9WgXcQ" --format json

# Show metadata for a bare video ID as a table
everything-cli google youtube metadata dQw4w9WgXcQ --format table

# Show metadata in TOON for an agent harness
everything-cli google youtube metadata "https://youtu.be/dQw4w9WgXcQ" --format toon`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := youtube.ParseVideoID(args[0])
			if err != nil {
				// The parse produced no ID, so carry the raw input (for bare
				// IDs this is the video ID itself) to keep the error lineage.
				return fmt.Errorf("video %s: %w", args[0], err)
			}
			p, err := client.Player(cmd.Context(), id)
			if err != nil {
				return fmt.Errorf("video %s: %w", id, err)
			}
			row := metadataRow(p)
			output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format), metadataFields, row, []map[string]any{row})
			return nil
		},
	}
}

// metadataRow maps a player response to its output row; empty player
// strings render as-is.
func metadataRow(p *youtube.Player) map[string]any {
	langs := make([]string, 0, len(p.Tracks))
	for _, t := range p.Tracks {
		langs = append(langs, t.Lang)
	}
	return map[string]any{
		"video_id":         p.VideoID,
		"title":            p.Title,
		"channel":          p.Author,
		"channel_id":       p.ChannelID,
		"duration_seconds": p.LengthSeconds,
		"view_count":       p.ViewCount,
		"publish_date":     p.PublishDate,
		"upload_date":      p.UploadDate,
		"category":         p.Category,
		"description":      p.Description,
		"available_langs":  langs,
	}
}
