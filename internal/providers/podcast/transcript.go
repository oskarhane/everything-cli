package podcast

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
	podcastapi "github.com/oskarhane/everything-cli/internal/podcast"
)

// transcriptFields is the transcript view field order for table output; the
// same names are the snake_case JSON and TOON keys. go-pretty's StyleLight
// upper-cases the headers when rendering.
var transcriptFields = []string{"show", "episode", "platform", "episode_id", "segments"}

// transcriptView is the structured transcript report for one episode: show
// metadata plus the timed segments of the transcript. The segments marshal
// directly as podcast.Segment, whose json tags provide the snake_case JSON
// and TOON keys.
type transcriptView struct {
	Show      string               `json:"show"`
	Episode   string               `json:"episode"`
	Platform  string               `json:"platform"`
	EpisodeID string               `json:"episode_id"`
	Segments  []podcastapi.Segment `json:"segments"`
}

// newTranscriptCmd returns `podcast transcript`: the timed transcript of one
// episode, found by an Apple Podcasts or Spotify episode URL. By default the
// transcript text streams to stdout as plain lines so the command pipes like
// any text writer; an explicit --format renders the structured report
// instead, and --raw forces the plain-text form even on a terminal. --out
// sends the plain text to a file.
func newTranscriptCmd(cfg *app.Config, client podcastapi.Client) *cobra.Command {
	var (
		raw bool
		out string
	)
	cmd := &cobra.Command{
		Use:   "transcript <url>",
		Short: "Print a podcast episode's timed transcript",
		Example: `# Stream an episode's transcript as plain text
everything-cli podcast transcript "https://podcasts.apple.com/us/podcast/slug/id123?i=456"

# Render the transcript with timings as JSON
everything-cli podcast transcript "https://open.spotify.com/episode/abc" --format json

# Save an episode's transcript to a file
everything-cli podcast transcript "https://podcasts.apple.com/us/podcast/slug/id123?i=456" --out notes.txt`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref, err := podcastapi.ParseURL(args[0])
			if err != nil {
				// The parse produced no ref, so carry the raw input to keep
				// the error lineage.
				return fmt.Errorf("episode %s: %w", args[0], err)
			}
			ctx := cmd.Context()
			ep, err := client.Episode(ctx, ref)
			if err != nil {
				return fmt.Errorf("episode %s: %w", ref.EpisodeID, err)
			}
			segs, err := client.Transcript(ctx, ep.TranscriptURL)
			if err != nil {
				return fmt.Errorf("episode %s: %w", ref.EpisodeID, err)
			}

			// writeRaw emits one transcript line per segment, streamed
			// plainly: no table/JSON framing. The text is still passed
			// through output.StripControl first — transcript text is
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
			// instead of stdout", so it beats everything — a set --out is
			// never silently ignored. Otherwise an explicit --format renders
			// the structured report; --raw, and any piped stdout, stream
			// plain text; only an interactive terminal gets the
			// auto-detected report.
			switch {
			case out != "":
				return streamRaw()
			case cfg.Format != "":
				printTranscript(cmd, cfg, ep, segs)
			case raw || !output.StdoutIsTerminal():
				return streamRaw()
			default:
				printTranscript(cmd, cfg, ep, segs)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&raw, "raw", false, "Print the transcript text as plain text instead of the structured report")
	cmd.Flags().StringVar(&out, "out", "", "Write the transcript text to this file instead of stdout")
	return cmd
}

// printTranscript renders the structured report: an object in JSON/TOON, a
// one-row table. JSON and TOON carry the full timed segments array; the table
// cell shows a compact count + total-duration summary instead of a raw struct
// dump.
func printTranscript(cmd *cobra.Command, cfg *app.Config, ep *podcastapi.Episode, segs []podcastapi.Segment) {
	view := transcriptView{
		Show:      ep.Show,
		Episode:   ep.Title,
		Platform:  ep.Platform,
		EpisodeID: ep.EpisodeID,
		Segments:  segs,
	}
	row := map[string]any{
		"show":       view.Show,
		"episode":    view.Episode,
		"platform":   view.Platform,
		"episode_id": view.EpisodeID,
		"segments":   segmentsSummary(segs),
	}
	output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format), transcriptFields, view, []map[string]any{row})
}

// segmentsSummary is the table cell for the segments column: a compact "N
// segments · MM:SS" (or "H:MM:SS" past an hour) from the segments' total
// duration. The full timed list lives in the JSON/TOON view instead.
func segmentsSummary(segs []podcastapi.Segment) string {
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
