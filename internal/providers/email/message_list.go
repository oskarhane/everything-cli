package email

import (
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
)

// messageListFields is the table column order; the same snake_case names
// are the JSON and TOON keys (AGENTS.md casing rule). go-pretty's
// StyleLight upper-cases the header cells at render time.
var messageListFields = []string{"uid", "date", "from", "subject", "flags"}

// newMessageListCmd returns `email message list`: envelope headers (never
// bodies) from one mailbox, newest first, capped by --limit. The list view
// is the cheap IMAP header fetch a user scans before fetching one message
// in full, so it defaults to the INBOX and a page of 25.
func newMessageListCmd(cfg *app.Config) *cobra.Command {
	var (
		mailbox string
		limit   int
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List message envelopes in a mailbox, newest first",
		Example: `# List the 25 most recent inbox messages as a table
everything-cli email message list --format table

# List at most 10 envelopes from the archive mailbox as JSON
everything-cli email message list --mailbox Archive --limit 10 --format json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, err := dialMail(cmd.Context(), cfg)
			if err != nil {
				return err
			}
			// Close releases the IMAP connection; dial succeeded, so the
			// leaf owns it until RunE returns.
			defer svc.Close()
			lister, err := As[EnvelopeLister](svc, nil)
			if err != nil {
				return err
			}
			envelopes, err := lister.ListEnvelopes(cmd.Context(), mailbox, limit)
			if err != nil {
				return err
			}
			printEnvelopeList(cmd, cfg, envelopes)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&mailbox, "mailbox", "INBOX", "Mailbox (folder) to list")
	f.IntVar(&limit, "limit", 25, "Maximum envelopes to return (<= 0 means all)")
	return cmd
}

// printEnvelopeList renders envelopes in the resolved format. JSON and
// TOON keep flags as an array (machine-readable); the table joins them
// into one cell so every row stays a single line.
func printEnvelopeList(cmd *cobra.Command, cfg *app.Config, envelopes []Envelope) {
	rows := make([]map[string]any, 0, len(envelopes))
	tableRows := make([]map[string]any, 0, len(envelopes))
	for _, e := range envelopes {
		date := e.Date.UTC().Format(time.RFC3339)
		rows = append(rows, map[string]any{
			"uid":     e.UID,
			"date":    date,
			"from":    e.From,
			"subject": e.Subject,
			"flags":   e.Flags,
		})
		tableRows = append(tableRows, map[string]any{
			"uid":     e.UID,
			"date":    date,
			"from":    e.From,
			"subject": e.Subject,
			"flags":   strings.Join(e.Flags, ","),
		})
	}
	output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format), messageListFields, rows, tableRows)
}
