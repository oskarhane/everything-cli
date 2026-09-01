package message

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/google/gmail/service"
)

// newUntrashCmd returns `gmail message untrash`: restore a trashed message to
// the mailbox.
func newUntrashCmd(_ *app.Config, newSvc service.Dialer[service.MessageService]) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "untrash <id>",
		Short: "Restore a Gmail message from trash",
		Example: `# Restore a trashed message
everything-cli gmail message untrash 19c2a4b7

# Restore a trashed message on another account
everything-cli gmail message untrash 19c2a4b7 --account work`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			if _, err := svc.UntrashMessage(cmd.Context(), args[0]); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Untrashed message %s\n", args[0]); err != nil {
				return err
			}
			return nil
		},
	}
	return cmd
}
