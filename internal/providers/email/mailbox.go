package email

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
)

// newMailboxCmd builds the `email mailbox` parent: read-only inspection of
// the acting account's IMAP mailboxes (folders). Each leaf lives in its own
// file and is wired here with one AddCommand line (AGENTS.md layout rule);
// message-level operations live under the sibling `email message` subtree,
// not here.
func newMailboxCmd(cfg *app.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mailbox",
		Short: "List the account's IMAP mailboxes (folders)",
	}
	cmd.AddCommand(newMailboxListCmd(cfg))
	return cmd
}
