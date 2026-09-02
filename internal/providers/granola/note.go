package granola

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
)

// newNoteCmd returns the `granola note` parent with every note leaf
// attached, one AddCommand line each. There is deliberately no search leaf:
// the public API documents no search endpoint.
func newNoteCmd(cfg *app.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "note",
		Short: "Read Granola notes",
	}
	cmd.AddCommand(newNoteListCmd(cfg))
	cmd.AddCommand(newNoteGetCmd(cfg))
	return cmd
}
