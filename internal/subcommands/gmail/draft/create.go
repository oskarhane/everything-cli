package draft

import (
	"encoding/base64"
	"fmt"

	"github.com/spf13/cobra"

	gmail "google.golang.org/api/gmail/v1"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/subcommands/gmail/message"
)

// newCreateCmd returns `gmail draft create`: compose a message and store it
// as a draft without sending. The body comes from exactly one of --body or
// --body-file; the raw message goes through the same shared BuildMIME
// pipeline `gmail message send` uses.
func newCreateCmd(cfg *app.Config, newSvc serviceFunc) *cobra.Command {
	var (
		to       string
		subject  string
		body     string
		bodyFile string
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a Gmail draft without sending it",
		Example: `# Create a draft from inline text
google-cli gmail draft create --to alice@example.com --subject "Lunch" --body "Noon works"

# Create a draft reading the body from a file
google-cli gmail draft create --to a@example.com,b@example.com --subject "Report" \
  --body-file report.txt --format json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			recipients := message.SplitCSV(to)
			if len(recipients) == 0 {
				return fmt.Errorf("no recipients: pass --to")
			}
			bodyText, err := message.ResolveBody(cfg.Fs, body, bodyFile)
			if err != nil {
				return err
			}
			raw, err := message.BuildMIME(cfg.Fs, recipients, nil, nil, subject, bodyText, nil)
			if err != nil {
				return err
			}
			svc, err := newDraftService(cmd.Context(), newSvc)
			if err != nil {
				return err
			}
			created, err := svc.CreateDraft(cmd.Context(), &gmail.Draft{
				Message: &gmail.Message{Raw: base64.RawURLEncoding.EncodeToString(raw)},
			})
			if err != nil {
				return err
			}
			printDraft(cmd, cfg, created)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&to, "to", "", "Comma-separated recipient addresses (required)")
	f.StringVar(&subject, "subject", "", "Subject line")
	f.StringVar(&body, "body", "", "Message body text (mutually exclusive with --body-file)")
	f.StringVar(&bodyFile, "body-file", "", "File to read the message body from (mutually exclusive with --body)")
	return cmd
}
