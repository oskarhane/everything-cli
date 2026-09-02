package youtube

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
	yt "github.com/oskarhane/everything-cli/internal/youtube"
)

// transcriptFields is the transcript view field order for table output; the
// same names are the snake_case JSON and TOON keys. go-pretty's StyleLight
// upper-cases the headers when rendering.
var transcriptFields = []string{"video_id", "title", "lang", "is_generated", "segments"}

// transcriptView is the structured transcript report for one video: player
// metadata plus the timed caption segments of the selected track. The
// segments marshal directly as yt.Segment, whose json tags provide the
// snake_case JSON and TOON keys.
type transcriptView struct {
	VideoID     string       `json:"video_id"`
	Title       string       `json:"title"`
	Lang        string       `json:"lang"`
	IsGenerated bool         `json:"is_generated"`
	Segments    []yt.Segment `json:"segments"`
}

// newTranscriptCmd returns `youtube transcript`: the timed captions of one
// video, found by watch URL, youtu.be / shorts / embed / live link, or bare
// video ID. By default the caption text streams to stdout as plain lines so
// the command pipes like any text writer (docs get's model); an explicit
// --format renders the structured report instead, and --raw forces the
// plain-text form even on a terminal. --out sends the plain text to a file.
func newTranscriptCmd(cfg *app.Config, client yt.Client) *cobra.Command {
	var (
		lang string
		raw  bool
		out  string
	)
	cmd := &cobra.Command{
		Use:   "transcript <url-or-id>",
		Short: "Print a video's timed transcript",
		Example: `# Stream a video's captions as plain text
everything-cli google youtube transcript https://www.youtube.com/watch?v=dQw4w9WgXcQ

# Render the transcript with timings as JSON
everything-cli google youtube transcript dQw4w9WgXcQ --format json

# Save a German caption track to a file
everything-cli google youtube transcript dQw4w9WgXcQ --lang de --out de.txt`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := yt.ParseVideoID(args[0])
			if err != nil {
				// The parse produced no ID, so carry the raw input (for bare
				// IDs this is the video ID itself) to keep the error lineage.
				return fmt.Errorf("video %s: %w", args[0], err)
			}
			ctx := cmd.Context()
			player, err := client.Player(ctx, id)
			if err != nil {
				return fmt.Errorf("video %s: %w", id, err)
			}
			track, err := yt.SelectTrack(player.Tracks, lang)
			if err != nil {
				return fmt.Errorf("video %s: %w", id, err)
			}
			segs, err := client.Transcript(ctx, track.BaseURL)
			if err != nil {
				return fmt.Errorf("video %s: %w", id, err)
			}

			// writeRaw emits one caption line per segment, streamed plainly
			// like docs get: no table/JSON framing. The text is still passed
			// through output.StripControl first — caption text is
			// creator-controlled, so control bytes (ANSI escapes, OSC 52)
			// must be neutralized before they reach the user's terminal.
			writeRaw := func(w io.Writer) error {
				for _, seg := range segs {
					if _, err := fmt.Fprintln(w, output.StripControl(seg.Text)); err != nil {
						return err
					}
				}
				return nil
			}
			// streamRaw sends the plain text to --out's file, or stdout.
			streamRaw := func() error {
				if out != "" {
					return app.WriteToFile(cfg.Fs, out, writeRaw)
				}
				if err := writeRaw(cmd.OutOrStdout()); err != nil {
					return fmt.Errorf("writing transcript to stdout: %w", err)
				}
				return nil
			}

			// Render priority: --out always means "plain text to this file
			// instead of stdout" (docs get's model), so it beats everything —
			// a set --out is never silently ignored. Otherwise an explicit
			// --format renders the structured report; --raw, and any piped
			// stdout, stream plain text; only an interactive terminal gets
			// the auto-detected report.
			switch {
			case out != "":
				return streamRaw()
			case cfg.Format != "":
				printTranscript(cmd, cfg, player, track, segs)
			case raw || !output.StdoutIsTerminal():
				return streamRaw()
			default:
				printTranscript(cmd, cfg, player, track, segs)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&lang, "lang", "en", "Caption track language (ISO 639-1 code)")
	cmd.Flags().BoolVar(&raw, "raw", false, "Print the caption text as plain text instead of the structured report")
	cmd.Flags().StringVar(&out, "out", "", "Write the caption text to this file instead of stdout")
	return cmd
}

// printTranscript renders the structured report: an object in JSON/TOON, a
// one-row table. JSON and TOON carry the full timed segments array; the table
// cell shows a compact count + total-duration summary instead of a raw struct
// dump.
func printTranscript(cmd *cobra.Command, cfg *app.Config, player *yt.Player, track yt.Track, segs []yt.Segment) {
	view := transcriptView{
		VideoID:     player.VideoID,
		Title:       player.Title,
		Lang:        track.Lang,
		IsGenerated: track.Generated,
		Segments:    segs,
	}
	row := map[string]any{
		"video_id":     view.VideoID,
		"title":        view.Title,
		"lang":         view.Lang,
		"is_generated": view.IsGenerated,
		"segments":     segmentsSummary(segs),
	}
	output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format), transcriptFields, view, []map[string]any{row})
}

// segmentsSummary is the table cell for the segments column: a compact "N
// segments · MM:SS" (or "H:MM:SS" past an hour) from the segments' total
// duration. The full timed list lives in the JSON/TOON view instead.
func segmentsSummary(segs []yt.Segment) string {
	var ms int64
	for _, s := range segs {
		ms += s.DurationMS
	}
	secs := ms / 1000
	h, m, s := secs/3600, secs%3600/60, secs%60
	if h > 0 {
		return fmt.Sprintf("%d segments · %d:%02d:%02d", len(segs), h, m, s)
	}
	return fmt.Sprintf("%d segments · %02d:%02d", len(segs), m, s)
}
