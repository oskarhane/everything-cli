package email

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
)

// sentConfirmation is the rendered shape of a successful send: a boolean and
// the recipient list only. Credentials, the subject, and the body are never
// echoed back — output must never carry anything an attacker could scrape.
type sentConfirmation struct {
	Sent bool     `json:"sent"`
	To   []string `json:"to"`
}

// newMessageSendCmd builds `email message send`: submit one plain-text
// message over SMTP through the acting account. The body comes from exactly
// one of --body or --body-file ("-" reads stdin, so scripts can pipe); all
// flag validation happens before the dial so a usage error never opens a
// connection.
func newMessageSendCmd(cfg *app.Config) *cobra.Command {
	var (
		to       []string
		cc       []string
		subject  string
		body     string
		bodyFile string
	)
	cmd := &cobra.Command{
		Use:   "send",
		Short: "Send a plain-text email via the account's SMTP server",
		Example: `# Send an inline message to one recipient
everything-cli email message send --to alice@example.com --subject "Lunch" --body "Noon works"

# Send to several recipients with cc, reading the body from a file
everything-cli email message send --to a@example.com --to b@example.com --cc carol@example.com \
  --subject "Report" --body-file report.txt

# Pipe the body from stdin
printf 'hi' | everything-cli email message send --to alice@example.com --subject "Hi" --body-file -`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if len(to) == 0 {
				return fmt.Errorf("no recipients: pass at least one --to")
			}
			if subject == "" {
				return fmt.Errorf("no subject: pass --subject")
			}
			bodyReader, err := resolveSendBody(cmd, cfg, body, bodyFile)
			if err != nil {
				return err
			}
			svc, err := dialMail(cmd.Context(), cfg)
			if err != nil {
				return err
			}
			// Close logs out of IMAP and releases the connection; the send
			// is already submitted by then, so a Close error is not fatal.
			defer func() { _ = svc.Close() }()
			sender, err := As[MessageSender](svc)
			if err != nil {
				return err
			}
			if err := sender.SendMessage(cmd.Context(), SendInput{
				To:      to,
				Cc:      cc,
				Subject: subject,
				Body:    bodyReader,
			}); err != nil {
				return err
			}
			view := sentConfirmation{Sent: true, To: to}
			output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format),
				[]string{"sent", "to"}, view,
				[]map[string]any{{"sent": view.Sent, "to": strings.Join(to, ", ")}})
			return nil
		},
	}
	f := cmd.Flags()
	f.StringArrayVar(&to, "to", nil, "Recipient address (repeatable, at least one required)")
	f.StringArrayVar(&cc, "cc", nil, "Cc address (repeatable)")
	f.StringVar(&subject, "subject", "", "Subject line (required)")
	f.StringVar(&body, "body", "", "Message body text (mutually exclusive with --body-file)")
	f.StringVar(&bodyFile, "body-file", "", "File to read the message body from, or - for stdin (mutually exclusive with --body)")
	return cmd
}

// resolveSendBody returns the message body from exactly one source: --body
// or --body-file ("-" means stdin). Both set, or neither, is a usage error —
// silently picking one would risk sending the wrong text. The body is read
// eagerly so a file/stdin failure surfaces before the dial.
func resolveSendBody(cmd *cobra.Command, cfg *app.Config, body, bodyFile string) (io.Reader, error) {
	switch {
	case body != "" && bodyFile != "":
		return nil, fmt.Errorf("--body and --body-file are mutually exclusive: pass exactly one")
	case bodyFile == "-":
		content, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return nil, fmt.Errorf("reading body from stdin: %w", err)
		}
		return bytes.NewReader(content), nil
	case bodyFile != "":
		content, err := afero.ReadFile(cfg.Fs, bodyFile)
		if err != nil {
			return nil, fmt.Errorf("reading --body-file: %w", err)
		}
		return bytes.NewReader(content), nil
	case body != "":
		return strings.NewReader(body), nil
	default:
		return nil, fmt.Errorf("no message body: pass exactly one of --body or --body-file")
	}
}
