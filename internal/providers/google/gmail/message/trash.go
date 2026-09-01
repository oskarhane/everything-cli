package message

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/google/gmail/service"
)

// newTrashCmd returns `gmail message trash`: move a message to TRASH. Trashing
// is recoverable (see untrash), unlike delete.
func newTrashCmd(_ *app.Config, newSvc service.Dialer[service.MessageService]) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trash <id>",
		Short: "Move a Gmail message to trash",
		Example: `# Trash a message
everything-cli gmail message trash 19c2a4b7

# Trash a message on another account
everything-cli gmail message trash 19c2a4b7 --account work`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			if _, err := svc.TrashMessage(cmd.Context(), args[0]); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Trashed message %s\n", args[0]); err != nil {
				return err
			}
			return nil
		},
	}
	return cmd
}
