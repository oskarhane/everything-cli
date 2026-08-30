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
)

// attachment is one file staged for a multipart message.
type attachment struct {
	name    string
	content []byte
}

// ResolveBody returns the message body from exactly one source: --body or
// --body-file. Both set, or neither, is an error. Every compose leaf
// (`gmail message send`, `gmail draft create`) resolves its body through
// this, so the two flags behave identically across the CLI.
func ResolveBody(fs afero.Fs, body, bodyFile string) (string, error) {
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

// BuildMIME composes the RFC 2822 message shared by every Gmail compose path
// (`gmail message send`, `gmail draft create`): a flat text/plain message
// without attachments, or a multipart/mixed message whose first part is the
// text body and whose remaining parts carry the attachments, each read from
// fs by path and typed by extension.
func BuildMIME(fs afero.Fs, to, cc, bcc []string, subject, body string, attachmentPaths []string) ([]byte, error) {
	files, err := readAttachments(fs, attachmentPaths)
	if err != nil {
		return nil, err
	}
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
	if len(files) == 0 {
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
	for _, file := range files {
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
