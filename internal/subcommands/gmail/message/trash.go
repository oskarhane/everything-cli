package message

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
)

// newTrashCmd returns `gmail message trash`: move a message to TRASH. Trashing
// is recoverable (see untrash), unlike delete.
func newTrashCmd(_ *app.Config, newSvc serviceFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trash <id>",
		Short: "Move a Gmail message to trash",
		Example: `# Trash a message
google-cli gmail message trash 19c2a4b7

# Trash a message on another account
google-cli gmail message trash 19c2a4b7 --account work`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newMessageService(cmd.Context(), newSvc)
			if err != nil {
				return err
			}
			if _, err := svc.TrashMessage(cmd.Context(), args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Trashed message %s\n", args[0])
			return nil
		},
	}
	return cmd
}
