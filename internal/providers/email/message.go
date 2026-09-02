package email

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
)

// newMessageCmd builds the `email message` parent: operations on messages
// inside a mailbox. Per the one-file-per-leaf layout, this file holds no
// leaf logic — each leaf (list, get, send) lives in its own file and is
// wired here with exactly one AddCommand line.
func newMessageCmd(cfg *app.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "message",
		Short: "List, read, and send messages in a mailbox",
	}
	cmd.AddCommand(newMessageGetCmd(cfg))
	cmd.AddCommand(newMessageListCmd(cfg))
	cmd.AddCommand(newMessageSendCmd(cfg))
	return cmd
}
