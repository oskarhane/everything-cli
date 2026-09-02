package email

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
)

// messageGetFields is the header field order for `email message get` table
// output. The same names are the snake_case JSON/TOON keys; go-pretty's
// StyleLight upper-cases the headers when rendering.
var messageGetFields = []string{"uid", "from", "to", "subject", "date"}

// messageGetView is the JSON/TOON shape of `email message get`: the full
// message minus the raw Headers map — the fields users read one message
// for are already surfaced individually, and attachment metadata stands in
// for the bytes (never fetched by this leaf).
type messageGetView struct {
	UID         uint32       `json:"uid"`
	From        string       `json:"from"`
	To          []string     `json:"to"`
	Subject     string       `json:"subject"`
	Date        time.Time    `json:"date"`
	BodyText    string       `json:"body_text"`
	Attachments []Attachment `json:"attachments"`
}

// newMessageGetCmd builds `email message get`: one fully fetched message by
// IMAP UID. The UID is exactly one positional arg because IMAP UIDs are
// per-mailbox, so --mailbox (default INBOX) scopes the lookup.
func newMessageGetCmd(cfg *app.Config) *cobra.Command {
	var mailbox string
	cmd := &cobra.Command{
		Use:   "get <uid>",
		Short: "Show one email message by IMAP UID",
		Example: `# Show message 7 from the inbox as JSON
everything-cli email message get 7 --format json

# Show a message from another mailbox
everything-cli email message get 42 --mailbox Archive`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			uid, err := strconv.ParseUint(args[0], 10, 32)
			if err != nil {
				return fmt.Errorf("invalid uid %q: must be a non-negative integer", args[0])
			}
			svc, err := dialMail(cmd.Context(), cfg)
			if err != nil {
				return err
			}
			// Close on the full service (not the narrowed interface, which
			// hides it) so the IMAP logout always runs, error or not.
			defer func() { _ = svc.Close() }()
			getter, err := As[MessageGetter](svc, nil)
			if err != nil {
				return err
			}
			msg, err := getter.GetMessage(cmd.Context(), mailbox, uint32(uid))
			if err != nil {
				return err
			}
			printMessageGet(cmd, cfg, msg)
			return nil
		},
	}
	cmd.Flags().StringVar(&mailbox, "mailbox", "INBOX", "Mailbox (folder) to fetch the message from")
	return cmd
}

// printMessageGet renders one message in the resolved output format.
// JSON/TOON get the full view; table gets the header row plus the decoded
// text body as plain text below it.
func printMessageGet(cmd *cobra.Command, cfg *app.Config, msg *Message) {
	format := output.ResolveOutput(cfg.Format)
	row := map[string]any{
		"uid":     msg.UID,
		"from":    msg.From,
		"to":      strings.Join(msg.To, ", "),
		"subject": msg.Subject,
		"date":    msg.Date.Format(time.RFC3339),
	}
	if format == output.FormatTable {
		output.PrintTable(cmd.OutOrStdout(), messageGetFields, []map[string]any{row})
		// The body bypasses output.Print, so strip control bytes here
		// (same rule as gmail's --raw path): a hostile message must not
		// inject ANSI/OSC escapes into the terminal. "\t\n\r" survive.
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), output.StripControl(msg.BodyText))
		return
	}
	view := messageGetView{
		UID:         msg.UID,
		From:        msg.From,
		To:          msg.To,
		Subject:     msg.Subject,
		Date:        msg.Date,
		BodyText:    msg.BodyText,
		Attachments: msg.Attachments,
	}
	// Control bytes in body_text/subject are handled centrally: JSON escapes
	// them and PrintToon deep-strips them (falling back to JSON if toon
	// still rejects), so a data-driven marshal failure can never panic.
	output.Print(cmd.OutOrStdout(), format, messageGetFields, view, []map[string]any{row})
}
