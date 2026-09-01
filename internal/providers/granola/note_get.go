package granola

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
)

// newNoteGetCmd returns `granola note get`: one note by id, with its AI
// summary, attendees, and calendar event. --include-transcript inlines the
// transcript; the API answers 413 TRANSCRIPT_TOO_LARGE when it does not fit
// one response (its paged transcript endpoint is not supported yet), and
// the error says so.
func newNoteGetCmd(cfg *app.Config) *cobra.Command {
	var includeTranscript bool
	cmd := &cobra.Command{
		Use:   "get <note-id>",
		Short: "Show one Granola note with its AI summary",
		Example: `# Show a note's summary as JSON
everything-cli granola note get not_abc123def456 --format json

# Include the full transcript inline
everything-cli granola note get not_abc123def456 --include-transcript`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := dialNotes(cmd.Context(), cfg)
			if err != nil {
				return err
			}
			note, err := svc.GetNote(cmd.Context(), args[0], includeTranscript)
			if err != nil {
				return err
			}
			return printNoteView(cmd, cfg, note)
		},
	}
	cmd.Flags().BoolVar(&includeTranscript, "include-transcript", false,
		"Inline the note's transcript (may fail with 413 TRANSCRIPT_TOO_LARGE on very long notes)")
	return cmd
}
