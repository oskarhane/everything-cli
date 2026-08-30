package message

import (
	"bytes"
	"encoding/base64"
	"fmt"
	mime "mime"
	"mime/multipart"
	"net/textproto"
	"path"
	"strings"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	gmail "google.golang.org/api/gmail/v1"

	"github.com/oskarhane/google-cli/internal/app"
)

// attachment is one file staged for a multipart message.
type attachment struct {
	name    string
	content []byte
}

// newSendCmd returns `gmail message send`: compose and send a message. The
// body comes from exactly one of --body or --body-file; attachments are read
// from disk and carried as base64 multipart parts.
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
			recipients := splitCSV(to)
			if len(recipients) == 0 {
				return fmt.Errorf("no recipients: pass --to (optionally --cc and --bcc)")
			}
			bodyText, err := resolveBody(cfg.Fs, body, bodyFile)
			if err != nil {
				return err
			}
			files, err := readAttachments(cfg.Fs, attachments)
			if err != nil {
				return err
			}
			raw, err := buildMIME(recipients, splitCSV(cc), splitCSV(bcc), subject, bodyText, files)
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

// readAttachments loads every attachment path from fs, naming each part after
// its base name and typing it by extension.
func readAttachments(fs afero.Fs, paths []string) ([]attachment, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	files := make([]attachment, 0, len(paths))
	for _, p := range paths {
		content, err := afero.ReadFile(fs, p)
		if err != nil {
			return nil, fmt.Errorf("reading --attachment %s: %w", p, err)
		}
		files = append(files, attachment{name: path.Base(p), content: content})
	}
	return files, nil
}

// buildMIME composes the RFC 2822 message: a flat text/plain message without
// attachments, or a multipart/mixed message whose first part is the text body
// and whose remaining parts carry the attachments as base64.
func buildMIME(to, cc, bcc []string, subject, body string, attachments []attachment) ([]byte, error) {
	var buf bytes.Buffer
	writeHeader(&buf, "To", strings.Join(to, ", "))
	if len(cc) > 0 {
		writeHeader(&buf, "Cc", strings.Join(cc, ", "))
	}
	if len(bcc) > 0 {
		writeHeader(&buf, "Bcc", strings.Join(bcc, ", "))
	}
	writeHeader(&buf, "Subject", subject)
	writeHeader(&buf, "MIME-Version", "1.0")
	if len(attachments) == 0 {
		writeHeader(&buf, "Content-Type", "text/plain; charset=UTF-8")
		buf.WriteString("\r\n")
		buf.WriteString(body)
		return buf.Bytes(), nil
	}
	w := multipart.NewWriter(&buf)
	writeHeader(&buf, "Content-Type", "multipart/mixed; boundary="+w.Boundary())
	// The writer's first part starts directly with "--boundary", so the blank
	// line ending the header block is written here.
	buf.WriteString("\r\n")
	textPart, err := w.CreatePart(textproto.MIMEHeader{"Content-Type": {"text/plain; charset=UTF-8"}})
	if err != nil {
		return nil, fmt.Errorf("composing text part: %w", err)
	}
	if _, err := textPart.Write([]byte(body)); err != nil {
		return nil, fmt.Errorf("writing text part: %w", err)
	}
	for _, file := range attachments {
		part, err := w.CreatePart(textproto.MIMEHeader{
			"Content-Type":              {contentType(file.name) + `; name="` + file.name + `"`},
			"Content-Disposition":       {`attachment; filename="` + file.name + `"`},
			"Content-Transfer-Encoding": {"base64"},
		})
		if err != nil {
			return nil, fmt.Errorf("composing attachment part %s: %w", file.name, err)
		}
		if _, err := part.Write([]byte(base64.StdEncoding.EncodeToString(file.content))); err != nil {
			return nil, fmt.Errorf("writing attachment part %s: %w", file.name, err)
		}
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("closing multipart message: %w", err)
	}
	return buf.Bytes(), nil
}

// contentType guesses an attachment's MIME type from its extension, defaulting
// to a generic binary type.
func contentType(name string) string {
	if t := mime.TypeByExtension(path.Ext(name)); t != "" {
		return strings.SplitN(t, ";", 2)[0]
	}
	return "application/octet-stream"
}

// writeHeader writes one RFC 2822 header line.
func writeHeader(buf *bytes.Buffer, name, value string) {
	fmt.Fprintf(buf, "%s: %s\r\n", name, value)
}
