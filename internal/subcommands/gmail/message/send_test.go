package message

import (
	"bufio"
	"encoding/base64"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/textproto"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// mimePart is one decoded part of a constructed MIME message.
type mimePart struct {
	header textproto.MIMEHeader
	body   string
}

// decodeSent returns the RFC 2822 message the leaf last handed the service,
// decoded from the API's base64url raw field, and asserts the wire encoding.
func decodeSent(t *testing.T, svc *fakeService) string {
	t.Helper()
	require.NotNil(t, svc.sent, "send must reach the API")
	require.NotEmpty(t, svc.sent.Raw)
	decoded, err := base64.RawURLEncoding.DecodeString(svc.sent.Raw)
	require.NoError(t, err, "raw field must be unpadded base64url")
	return string(decoded)
}

// splitMIME splits a message into its header block and body.
func splitMIME(t *testing.T, raw string) (textproto.MIMEHeader, string) {
	t.Helper()
	headerBlock, body, ok := strings.Cut(raw, "\r\n\r\n")
	require.True(t, ok, "message must have a header/body split: %q", raw)
	// The appended blank line terminates the header block the way it appears
	// on the wire, which ReadMIMEHeader needs to stop cleanly.
	header, err := textproto.NewReader(bufio.NewReader(strings.NewReader(headerBlock + "\r\n\r\n"))).ReadMIMEHeader()
	require.NoError(t, err)
	return header, body
}

// readParts returns every part of a multipart body.
func readParts(t *testing.T, body, boundary string) []mimePart {
	t.Helper()
	r := multipart.NewReader(strings.NewReader(body), boundary)
	var parts []mimePart
	for {
		part, err := r.NextPart()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		content, err := io.ReadAll(part)
		require.NoError(t, err)
		parts = append(parts, mimePart{header: part.Header, body: string(content)})
	}
	return parts
}

func TestSendPlainMIME(t *testing.T) {
	svc := &fakeService{}
	runCmd(t, newLeafCmd(newSendCmd, svc, "json"),
		"--to", "alice@example.com, bob@example.com",
		"--cc", "carol@example.com",
		"--bcc", "dave@example.com",
		"--subject", "Lunch",
		"--body", "Noon works",
	)

	header, body := splitMIME(t, decodeSent(t, svc))
	require.Equal(t, "alice@example.com, bob@example.com", header.Get("To"))
	require.Equal(t, "carol@example.com", header.Get("Cc"))
	require.Equal(t, "dave@example.com", header.Get("Bcc"))
	require.Equal(t, "Lunch", header.Get("Subject"))
	require.Equal(t, "1.0", header.Get("MIME-Version"))
	require.Contains(t, header.Get("Content-Type"), "text/plain")
	require.NotContains(t, header.Get("Content-Type"), "multipart")
	require.Equal(t, "Noon works", body)
}

func TestSendMultipartMIME(t *testing.T) {
	cfg := newTestConfig("json")
	require.NoError(t, afero.WriteFile(cfg.Fs, "notes.txt", []byte("the body text"), 0o644))
	require.NoError(t, afero.WriteFile(cfg.Fs, "data.csv", []byte("a,b\n1,2\n"), 0o644))
	svc := &fakeService{}

	runCmd(t, newSendCmd(cfg, fakeNewSvc(svc)),
		"--to", "alice@example.com",
		"--subject", "Report",
		"--body-file", "notes.txt",
		"--attachment", "data.csv",
	)

	header, body := splitMIME(t, decodeSent(t, svc))
	mediaType, params, err := mime.ParseMediaType(header.Get("Content-Type"))
	require.NoError(t, err)
	require.Equal(t, "multipart/mixed", mediaType)

	parts := readParts(t, body, params["boundary"])
	require.Len(t, parts, 2, "text part + one attachment part")

	require.Equal(t, "text/plain; charset=UTF-8", parts[0].header.Get("Content-Type"))
	require.Equal(t, "the body text", parts[0].body)

	file := parts[1]
	require.Contains(t, file.header.Get("Content-Disposition"), `attachment; filename="data.csv"`)
	require.Contains(t, file.header.Get("Content-Type"), "text/csv")
	require.Equal(t, "base64", file.header.Get("Content-Transfer-Encoding"))
	decoded, err := base64.StdEncoding.DecodeString(file.body)
	require.NoError(t, err, "attachment content must be base64")
	require.Equal(t, "a,b\n1,2\n", string(decoded))
}

func TestSendRepeatableAttachments(t *testing.T) {
	cfg := newTestConfig("json")
	require.NoError(t, afero.WriteFile(cfg.Fs, "one.txt", []byte("one"), 0o644))
	require.NoError(t, afero.WriteFile(cfg.Fs, "two.txt", []byte("two"), 0o644))
	svc := &fakeService{}

	runCmd(t, newSendCmd(cfg, fakeNewSvc(svc)),
		"--to", "alice@example.com",
		"--body", "b",
		"--attachment", "one.txt",
		"--attachment", "two.txt",
	)

	header, body := splitMIME(t, decodeSent(t, svc))
	_, params, err := mime.ParseMediaType(header.Get("Content-Type"))
	require.NoError(t, err)
	require.Len(t, readParts(t, body, params["boundary"]), 3, "text + two attachments")
}

func TestSendBodyFile(t *testing.T) {
	cfg := newTestConfig("json")
	require.NoError(t, afero.WriteFile(cfg.Fs, "note.txt", []byte("file body"), 0o644))
	svc := &fakeService{}

	runCmd(t, newSendCmd(cfg, fakeNewSvc(svc)),
		"--to", "alice@example.com", "--body-file", "note.txt")

	_, body := splitMIME(t, decodeSent(t, svc))
	require.Equal(t, "file body", body)
}

func TestSendRefusesAmbiguousBodyFlags(t *testing.T) {
	cfg := newTestConfig("json")
	require.NoError(t, afero.WriteFile(cfg.Fs, "note.txt", []byte("file body"), 0o644))
	svc := &fakeService{}

	_, err := runCmdErr(t, newSendCmd(cfg, fakeNewSvc(svc)),
		"--to", "alice@example.com", "--body", "inline", "--body-file", "note.txt")

	require.Contains(t, err.Error(), "--body and --body-file are mutually exclusive")
	require.Nil(t, svc.sent, "ambiguous input must not reach the API")
}

func TestSendRefusesMissingBody(t *testing.T) {
	svc := &fakeService{}
	_, err := runCmdErr(t, newLeafCmd(newSendCmd, svc, "json"), "--to", "alice@example.com")

	require.Contains(t, err.Error(), "no message body")
	require.Nil(t, svc.sent)
}

func TestSendRequiresTo(t *testing.T) {
	svc := &fakeService{}
	_, err := runCmdErr(t, newLeafCmd(newSendCmd, svc, "json"), "--body", "hi")

	require.Contains(t, err.Error(), "no recipients")
	require.Nil(t, svc.sent)
}

func TestSendMissingAttachmentFile(t *testing.T) {
	svc := &fakeService{}
	_, err := runCmdErr(t, newLeafCmd(newSendCmd, svc, "json"),
		"--to", "alice@example.com", "--body", "hi", "--attachment", "nope.txt")

	require.Contains(t, err.Error(), "reading --attachment nope.txt")
	require.Nil(t, svc.sent)
}

func TestSendEchoesSentMessage(t *testing.T) {
	svc := &fakeService{}
	out := runCmd(t, newLeafCmd(newSendCmd, svc, "json"),
		"--to", "alice@example.com", "--body", "hi")

	row, ok := decodeJSON(t, out).(map[string]any)
	require.True(t, ok)
	require.Equal(t, "msg_99", row["id"])
	require.Equal(t, []any{"SENT"}, row["label_ids"])
}

func TestSendPropagatesAPIError(t *testing.T) {
	svc := &fakeService{err: errors.New("googleapi: Error 413: message too large")}
	_, err := runCmdErr(t, newLeafCmd(newSendCmd, svc, "json"),
		"--to", "alice@example.com", "--body", "hi")

	require.Contains(t, err.Error(), "googleapi: Error 413")
}
