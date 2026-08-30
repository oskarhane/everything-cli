package message

import (
	"encoding/base64"
	"fmt"

	"github.com/spf13/cobra"

	gmail "google.golang.org/api/gmail/v1"

	"github.com/oskarhane/google-cli/internal/app"
)

// newSendCmd returns `gmail message send`: compose and send a message. The
// body comes from exactly one of --body or --body-file; attachments are read
// from disk and carried as base64 multipart parts. The MIME composition is
// the shared BuildMIME pipeline the draft leaves use too.
func newSendCmd(cfg *app.Config, newSvc serviceFunc) *cobra.Command {
	var (
		to          string
		cc          string
		bcc         string
		subject     string
		body        string
		bodyFile    string
		attachments []string
	)
	cmd := &cobra.Command{
		Use:   "send",
		Short: "Send a Gmail message",
		Example: `# Send a plain-text message
google-cli gmail message send --to alice@example.com --subject "Lunch" --body "Noon works"

# Send a message with attachments, reading the body from a file
google-cli gmail message send --to a@example.com,b@example.com --subject "Report" \
  --body-file report.txt --attachment q1.pdf --attachment data.csv`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			recipients := SplitCSV(to)
			if len(recipients) == 0 {
				return fmt.Errorf("no recipients: pass --to (optionally --cc and --bcc)")
			}
			bodyText, err := ResolveBody(cfg.Fs, body, bodyFile)
			if err != nil {
				return err
			}
			raw, err := BuildMIME(cfg.Fs, recipients, SplitCSV(cc), SplitCSV(bcc), subject, bodyText, attachments)
			if err != nil {
				return err
			}
			svc, err := newMessageService(cmd.Context(), newSvc)
			if err != nil {
				return err
			}
			sent, err := svc.SendMessage(cmd.Context(), &gmail.Message{Raw: base64.RawURLEncoding.EncodeToString(raw)})
			if err != nil {
				return err
			}
			printMessage(cmd, cfg, sent)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&to, "to", "", "Comma-separated recipient addresses (required)")
	f.StringVar(&cc, "cc", "", "Comma-separated cc addresses")
	f.StringVar(&bcc, "bcc", "", "Comma-separated bcc addresses")
	f.StringVar(&subject, "subject", "", "Subject line")
	f.StringVar(&body, "body", "", "Message body text (mutually exclusive with --body-file)")
	f.StringVar(&bodyFile, "body-file", "", "File to read the message body from (mutually exclusive with --body)")
	f.StringArrayVar(&attachments, "attachment", nil, "File to attach (repeatable)")
	return cmd
}
