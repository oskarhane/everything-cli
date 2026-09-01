package granola

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
)

// newNoteListCmd returns `granola note list`: every note matching the
// API's filters, following cursor pagination across pages. Only notes with
// a generated AI summary and transcript are returned by the API.
func newNoteListCmd(cfg *app.Config) *cobra.Command {
	var opts ListOptions
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Granola notes",
		Example: `# List all notes as JSON
everything-cli granola note list --format json

# List notes created in a date range, as a table
everything-cli granola note list --created-after 2026-08-01 --created-before 2026-09-01 --format table

# List notes in a folder (includes child folders)
everything-cli granola note list --folder-id fol_abc123def456gh`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, err := dialNotes(cmd.Context(), cfg)
			if err != nil {
				return err
			}
			notes, err := svc.ListNotes(cmd.Context(), opts)
			if err != nil {
				return err
			}
			printNoteList(cmd, cfg, notes)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&opts.CreatedBefore, "created-before", "", "Only notes created before this date or date-time (e.g. 2026-09-01)")
	f.StringVar(&opts.CreatedAfter, "created-after", "", "Only notes created after this date or date-time")
	f.StringVar(&opts.UpdatedAfter, "updated-after", "", "Only notes updated after this date or date-time")
	f.StringVar(&opts.FolderID, "folder-id", "", "Only notes in this folder (fol_...; includes child folders)")
	return cmd
}
