package email

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
)

// mailboxFields is the mailbox field order for table output; the same names
// are the snake_case JSON and TOON keys. go-pretty's StyleLight upper-cases
// the header when rendering (AGENTS.md rule: tests assert "NAME").
var mailboxFields = []string{"name"}

// newMailboxListCmd returns `email mailbox list`: every mailbox on the
// acting account, in the server's sorted order. The leaf consumes only the
// MailboxLister surface of the dialed MailService so tests fake one method
// instead of the whole IMAP/SMTP union.
func newMailboxListCmd(cfg *app.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List mailboxes on the email account",
		Example: `# List mailboxes as JSON
everything-cli email mailbox list --format json

# List mailboxes as a table
everything-cli email mailbox list --format table`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, err := dialMail(cmd.Context(), cfg)
			if err != nil {
				return err
			}
			// Close logs out of IMAP; the connection must not outlive the
			// command even when listing fails.
			defer func() { _ = svc.Close() }()
			lister, err := As[MailboxLister](svc)
			if err != nil {
				return err
			}
			names, err := lister.ListMailboxes(cmd.Context())
			if err != nil {
				return err
			}
			printMailboxes(cmd, cfg, names)
			return nil
		},
	}
}

// printMailboxes renders zero or more mailbox names as one {"name": ...}
// row each, in the resolved output format.
func printMailboxes(cmd *cobra.Command, cfg *app.Config, names []string) {
	rows := make([]map[string]any, 0, len(names))
	for _, name := range names {
		rows = append(rows, map[string]any{"name": name})
	}
	output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format), mailboxFields, rows, rows)
}
