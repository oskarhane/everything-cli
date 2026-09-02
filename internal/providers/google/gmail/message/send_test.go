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

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
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
	cmdtest.RunCmd(t, newLeafCmd(newSendCmd, svc, "json"),
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
	cfg := cmdtest.NewTestConfig("json")
	require.NoError(t, afero.WriteFile(cfg.Fs, "notes.txt", []byte("the body text"), 0o644))
	require.NoError(t, afero.WriteFile(cfg.Fs, "data.csv", []byte("a,b\n1,2\n"), 0o644))
	svc := &fakeService{}

	cmdtest.RunCmd(t, newSendCmd(cfg, fakeNewSvc(svc)),
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
	cfg := cmdtest.NewTestConfig("json")
	require.NoError(t, afero.WriteFile(cfg.Fs, "one.txt", []byte("one"), 0o644))
	require.NoError(t, afero.WriteFile(cfg.Fs, "two.txt", []byte("two"), 0o644))
	svc := &fakeService{}

	cmdtest.RunCmd(t, newSendCmd(cfg, fakeNewSvc(svc)),
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
	cfg := cmdtest.NewTestConfig("json")
	require.NoError(t, afero.WriteFile(cfg.Fs, "note.txt", []byte("file body"), 0o644))
	svc := &fakeService{}

	cmdtest.RunCmd(t, newSendCmd(cfg, fakeNewSvc(svc)),
		"--to", "alice@example.com", "--body-file", "note.txt")

	_, body := splitMIME(t, decodeSent(t, svc))
	require.Equal(t, "file body", body)
}

func TestSendRefusesAmbiguousBodyFlags(t *testing.T) {
	cfg := cmdtest.NewTestConfig("json")
	require.NoError(t, afero.WriteFile(cfg.Fs, "note.txt", []byte("file body"), 0o644))
	svc := &fakeService{}

	_, err := cmdtest.RunCmdErr(t, newSendCmd(cfg, fakeNewSvc(svc)),
		"--to", "alice@example.com", "--body", "inline", "--body-file", "note.txt")

	require.Contains(t, err.Error(), "--body and --body-file are mutually exclusive")
	require.Nil(t, svc.sent, "ambiguous input must not reach the API")
}

func TestSendRefusesMissingBody(t *testing.T) {
	svc := &fakeService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newSendCmd, svc, "json"), "--to", "alice@example.com")

	require.Contains(t, err.Error(), "no message body")
	require.Nil(t, svc.sent)
}

func TestSendRequiresTo(t *testing.T) {
	svc := &fakeService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newSendCmd, svc, "json"), "--body", "hi")

	require.Contains(t, err.Error(), "no recipients")
	require.Nil(t, svc.sent)
}

func TestSendMissingAttachmentFile(t *testing.T) {
	svc := &fakeService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newSendCmd, svc, "json"),
		"--to", "alice@example.com", "--body", "hi", "--attachment", "nope.txt")

	require.Contains(t, err.Error(), "reading --attachment nope.txt")
	require.Nil(t, svc.sent)
}

func TestSendEchoesSentMessage(t *testing.T) {
	svc := &fakeService{}
	out := cmdtest.RunCmd(t, newLeafCmd(newSendCmd, svc, "json"),
		"--to", "alice@example.com", "--body", "hi")

	row, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok)
	require.Equal(t, "msg_99", row["id"])
	require.Equal(t, []any{"SENT"}, row["label_ids"])
}

func TestSendPropagatesAPIError(t *testing.T) {
	svc := &fakeService{err: errors.New("googleapi: Error 413: message too large")}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newSendCmd, svc, "json"),
		"--to", "alice@example.com", "--body", "hi")

	require.Contains(t, err.Error(), "googleapi: Error 413")
}

// TestSendRefusesHeaderInjection pins the CRLF fail-closed behavior of the
// compose pipeline (findings S3 + S11): any --subject/--to/--cc/--bcc value
// carrying control characters is rejected before any MIME is built, so the
// API is never reached. A CR or LF in a header value would let a crafted
// value smuggle in extra headers, for example a forged Bcc recipient.
func TestSendRefusesHeaderInjection(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "subject with CRLF",
			args: []string{"--to", "alice@example.com", "--subject", "hi\r\nBcc: victim@evil.example", "--body", "hi"},
			want: `subject "hi\r\nBcc: victim@evil.example" contains control characters`,
		},
		{
			name: "subject with bare LF",
			args: []string{"--to", "alice@example.com", "--subject", "hi\nEvil: x", "--body", "hi"},
			want: `contains control characters`,
		},
		{
			name: "recipient with embedded CRLF",
			args: []string{"--to", "a@x.com,b\nc: evil", "--body", "hi"},
			want: `recipient "b\nc: evil" contains control characters`,
		},
		{
			name: "cc with embedded CRLF",
			args: []string{"--to", "a@x.com", "--cc", "b@x.com\r\nX-Evil: 1", "--body", "hi"},
			want: `recipient "b@x.com\r\nX-Evil: 1" contains control characters`,
		},
		{
			name: "bcc with embedded CRLF",
			args: []string{"--to", "a@x.com", "--bcc", "b@x.com\rBcc: victim@evil.example", "--body", "hi"},
			want: `recipient "b@x.com\rBcc: victim@evil.example" contains control characters`,
		},
		{
			name: "recipient with NUL",
			args: []string{"--to", "a@x.com,\x00@evil.example", "--body", "hi"},
			want: `contains control characters`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeService{}
			_, err := cmdtest.RunCmdErr(t, newLeafCmd(newSendCmd, svc, "json"), tt.args...)

			require.ErrorContains(t, err, tt.want)
			require.Nil(t, svc.sent, "rejected input must not reach the API")
		})
	}
}

// TestSendRefusesAttachmentNameInjection pins the fail-closed attachment-name
// policy (finding S11): a double quote closes the filename= quoting early and
// CR or LF breaks the part header, so such files are rejected outright.
func TestSendRefusesAttachmentNameInjection(t *testing.T) {
	cfg := cmdtest.NewTestConfig("json")
	require.NoError(t, afero.WriteFile(cfg.Fs, "evil\"quote.txt", []byte("data"), 0o644))
	svc := &fakeService{}

	_, err := cmdtest.RunCmdErr(t, newSendCmd(cfg, fakeNewSvc(svc)),
		"--to", "alice@example.com", "--body", "hi", "--attachment", "evil\"quote.txt")

	require.ErrorContains(t, err, "contains '\"'")
	require.ErrorContains(t, err, "rename the file")
	require.Nil(t, svc.sent, "rejected input must not reach the API")
}

// TestSendCleanUnicodeInputs passes unchanged: the controls rejection must not
// trip on ordinary unicode subjects or names.
func TestSendCleanUnicodeInputs(t *testing.T) {
	svc := &fakeService{}
	cmdtest.RunCmd(t, newLeafCmd(newSendCmd, svc, "json"),
		"--to", "olá@example.com, Björn@example.com",
		"--subject", "Résumé — sənd ✉",
		"--body", "C’est parti",
	)

	header, body := splitMIME(t, decodeSent(t, svc))
	require.Equal(t, "olá@example.com, Björn@example.com", header.Get("To"))
	require.Equal(t, "Résumé — sənd ✉", header.Get("Subject"))
	require.Equal(t, "C’est parti", body)
}
