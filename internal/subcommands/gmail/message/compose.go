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
	for _, r := range to {
		if err := validateHeaderValue("recipient", r); err != nil {
			return nil, err
		}
	}
	for _, r := range cc {
		if err := validateHeaderValue("recipient", r); err != nil {
			return nil, err
		}
	}
	for _, r := range bcc {
		if err := validateHeaderValue("recipient", r); err != nil {
			return nil, err
		}
	}
	if err := validateHeaderValue("subject", subject); err != nil {
		return nil, err
	}
	files, err := readAttachments(fs, attachmentPaths)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := writeHeader(&buf, "To", strings.Join(to, ", ")); err != nil {
		return nil, err
	}
	if len(cc) > 0 {
		if err := writeHeader(&buf, "Cc", strings.Join(cc, ", ")); err != nil {
			return nil, err
		}
	}
	if len(bcc) > 0 {
		if err := writeHeader(&buf, "Bcc", strings.Join(bcc, ", ")); err != nil {
			return nil, err
		}
	}
	if err := writeHeader(&buf, "Subject", subject); err != nil {
		return nil, err
	}
	if err := writeHeader(&buf, "MIME-Version", "1.0"); err != nil {
		return nil, err
	}
	if len(files) == 0 {
		if err := writeHeader(&buf, "Content-Type", "text/plain; charset=UTF-8"); err != nil {
			return nil, err
		}
		buf.WriteString("\r\n")
		buf.WriteString(body)
		return buf.Bytes(), nil
	}
	w := multipart.NewWriter(&buf)
	if err := writeHeader(&buf, "Content-Type", "multipart/mixed; boundary="+w.Boundary()); err != nil {
		return nil, err
	}
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
		name := path.Base(p)
		if err := validateAttachmentName(name); err != nil {
			return nil, fmt.Errorf("--attachment %s: %w", p, err)
		}
		files = append(files, attachment{name: name, content: content})
	}
	return files, nil
}

// validateAttachmentName rejects part names that cannot be carried safely in
// the Content-Type name= / Content-Disposition filename= quoted-strings: a
// double quote closes the quoting early and CR or LF breaks the part header
// itself. MVP policy: reject rather than RFC 2047-escape.
func validateAttachmentName(name string) error {
	for _, r := range name {
		switch r {
		case '"', '\r', '\n':
			return fmt.Errorf("file name %q contains %q; rename the file before attaching", name, r)
		}
	}
	return nil
}

// contentType guesses an attachment's MIME type from its extension, defaulting
// to a generic binary type.
func contentType(name string) string {
	if t := mime.TypeByExtension(path.Ext(name)); t != "" {
		return strings.SplitN(t, ";", 2)[0]
	}
	return "application/octet-stream"
}

// containsControl reports whether s contains a C0 control byte (NUL, CR, LF,
// tab, and the rest) — bytes that could break out of a header line.
func containsControl(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] == 0x7f {
			return true
		}
	}
	return false
}

// validateHeaderValue rejects a compose value that could break the RFC 2822
// header block. A CR or LF lets a crafted value smuggle in extra headers (for
// example a forged Bcc), so compose fails closed rather than stripping — a
// stripped subject could silently change meaning.
func validateHeaderValue(kind, value string) error {
	if containsControl(value) {
		return fmt.Errorf("%s %q contains control characters", kind, value)
	}
	return nil
}

// writeHeader writes one RFC 2822 header line, refusing any value that could
// break out of the header block.
func writeHeader(buf *bytes.Buffer, name, value string) error {
	if containsControl(value) {
		return fmt.Errorf("header %q contains control characters", value)
	}
	fmt.Fprintf(buf, "%s: %s\r\n", name, value)
	return nil
}
