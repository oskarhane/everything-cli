package draft

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	gmail "google.golang.org/api/gmail/v1"

	"github.com/oskarhane/google-cli/internal/app"
)

// newCreateCmd returns `gmail draft create`: compose a message and store it
// as a draft without sending. The body comes from exactly one of --body or
// --body-file; the raw message is built the same way `gmail message send`
// builds it.
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
			recipients := splitCSV(to)
			if len(recipients) == 0 {
				return fmt.Errorf("no recipients: pass --to")
			}
			bodyText, err := resolveBody(cfg.Fs, body, bodyFile)
			if err != nil {
				return err
			}
			raw, err := buildMIME(recipients, subject, bodyText)
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

// splitCSV splits a comma-separated flag value into trimmed, non-empty items,
// the CSV convention for the --to flag.
func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	var items []string
	for _, part := range strings.Split(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			items = append(items, p)
		}
	}
	return items
}

// resolveBody returns the message body from exactly one source: --body or
// --body-file. Both set, or neither, is an error.
func resolveBody(fs afero.Fs, body, bodyFile string) (string, error) {
	switch {
	case body != "" && bodyFile != "":
		return "", fmt.Errorf("--body and --body-file are mutually exclusive: pass exactly one")
	case bodyFile != "":
		content, err := afero.ReadFile(fs, bodyFile)
		if err != nil {
			return "", fmt.Errorf("reading --body-file: %w", err)
		}
		return string(content), nil
	case body != "":
		return body, nil
	default:
		return "", fmt.Errorf("no message body: pass exactly one of --body or --body-file")
	}
}

// buildMIME composes the flat text/plain RFC 2822 message a draft stores: the
// To and Subject headers, the MIME boilerplate, then the body. It mirrors
// message/send.go's no-attachment branch, kept local because sharing it would
// mean a new package for four lines.
func buildMIME(to []string, subject, body string) ([]byte, error) {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "To: %s\r\n", strings.Join(to, ", "))
	fmt.Fprintf(&buf, "Subject: %s\r\n", subject)
	fmt.Fprintf(&buf, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&buf, "Content-Type: text/plain; charset=UTF-8\r\n")
	buf.WriteString("\r\n")
	buf.WriteString(body)
	return buf.Bytes(), nil
}
