package email

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/providers/emailtest"
)

const (
	testIMAPUser     = "user@example.com"
	testIMAPPassword = "adapter-test-secret"
)

var (
	testSimpleDate    = time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	testMultipartDate = time.Date(2026, 6, 2, 11, 30, 0, 0, time.UTC)
)

// attachmentBytes is the decoded attachment payload the multipart fixture
// carries; the raw message holds it base64-encoded.
var attachmentBytes = []byte("fake pdf bytes")

func simpleRawMessage() string {
	return "MIME-Version: 1.0\r\n" +
		"From: Alice <alice@example.com>\r\n" +
		"To: bob@example.com\r\n" +
		"Subject: Hello there\r\n" +
		"Date: " + testSimpleDate.Format(time.RFC1123Z) + "\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"\r\n" +
		"Simple body\r\n"
}

func multipartRawMessage() string {
	return "MIME-Version: 1.0\r\n" +
		"From: Carol <carol@example.com>\r\n" +
		"To: bob@example.com\r\n" +
		"Subject: Report attached\r\n" +
		"Date: " + testMultipartDate.Format(time.RFC1123Z) + "\r\n" +
		"Content-Type: multipart/mixed; boundary=\"mix-boundary\"\r\n" +
		"\r\n" +
		"--mix-boundary\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"\r\n" +
		"See the attached report.\r\n" +
		"--mix-boundary\r\n" +
		"Content-Type: application/pdf; name=\"report.pdf\"\r\n" +
		"Content-Disposition: attachment; filename=\"report.pdf\"\r\n" +
		"Content-Transfer-Encoding: base64\r\n" +
		"\r\n" +
		base64.StdEncoding.EncodeToString(attachmentBytes) + "\r\n" +
		"--mix-boundary--\r\n"
}

// stubTLSRoots points the adapter's TLS config seam at the test CA for the
// test's lifetime, keeping full verification against the loopback server.
func stubTLSRoots(t *testing.T, roots *x509.CertPool) {
	t.Helper()
	saved := tlsConfigFor
	tlsConfigFor = func(host string) *tls.Config {
		cfg := saved(host)
		cfg.RootCAs = roots
		return cfg
	}
	t.Cleanup(func() { tlsConfigFor = saved })
}

// newSeededIMAP starts an in-process IMAP server with INBOX (one simple
// and one multipart message, the first flagged \Seen) and Archive (empty),
// and returns an adapter connected to it.
func newSeededIMAP(t *testing.T) *mailService {
	t.Helper()
	host, port, roots := emailtest.StartIMAP(t, testIMAPUser, testIMAPPassword, map[string][]emailtest.SeedMessage{
		"INBOX": {
			{Raw: simpleRawMessage(), Flags: []string{`\Seen`}, Time: testSimpleDate},
			{Raw: multipartRawMessage(), Time: testMultipartDate},
		},
		"Archive": {},
	})
	stubTLSRoots(t, roots)
	svc, err := newMailService(&credentials{
		Username: testIMAPUser,
		Password: testIMAPPassword,
		IMAP:     serverConfig{Host: host, Port: port},
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })
	return svc
}

func TestListMailboxes(t *testing.T) {
	svc := newSeededIMAP(t)

	names, err := svc.ListMailboxes(t.Context())
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"INBOX", "Archive"}, names)
}

func TestListEnvelopes(t *testing.T) {
	svc := newSeededIMAP(t)

	envelopes, err := svc.ListEnvelopes(t.Context(), "INBOX", 0)
	require.NoError(t, err)
	require.Len(t, envelopes, 2)

	// Newest first: the multipart message (UID 2) leads.
	assert.Equal(t, uint32(2), envelopes[0].UID)
	assert.Equal(t, "Carol <carol@example.com>", envelopes[0].From)
	assert.Equal(t, "Report attached", envelopes[0].Subject)
	assert.True(t, testMultipartDate.Equal(envelopes[0].Date), "date = %v", envelopes[0].Date)
	assert.Empty(t, envelopes[0].Flags)

	assert.Equal(t, uint32(1), envelopes[1].UID)
	assert.Equal(t, "Alice <alice@example.com>", envelopes[1].From)
	assert.Equal(t, "Hello there", envelopes[1].Subject)
	assert.True(t, testSimpleDate.Equal(envelopes[1].Date), "date = %v", envelopes[1].Date)
	assert.Equal(t, []string{`\Seen`}, envelopes[1].Flags)
}

func TestListEnvelopes_Limit(t *testing.T) {
	svc := newSeededIMAP(t)

	envelopes, err := svc.ListEnvelopes(t.Context(), "INBOX", 1)
	require.NoError(t, err)
	require.Len(t, envelopes, 1)
	assert.Equal(t, uint32(2), envelopes[0].UID)
}

func TestListEnvelopes_EmptyMailbox(t *testing.T) {
	svc := newSeededIMAP(t)

	envelopes, err := svc.ListEnvelopes(t.Context(), "Archive", 25)
	require.NoError(t, err)
	assert.Empty(t, envelopes)
}

func TestGetMessage(t *testing.T) {
	svc := newSeededIMAP(t)

	tests := []struct {
		name    string
		uid     uint32
		subject string
		from    string
		body    string
	}{
		{
			name:    "simple plain text",
			uid:     1,
			subject: "Hello there",
			from:    "Alice <alice@example.com>",
			body:    "Simple body",
		},
		{
			name:    "multipart with attachment",
			uid:     2,
			subject: "Report attached",
			from:    "Carol <carol@example.com>",
			body:    "See the attached report.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := svc.GetMessage(t.Context(), "INBOX", tt.uid)
			require.NoError(t, err)
			assert.Equal(t, tt.uid, msg.UID)
			assert.Equal(t, tt.subject, msg.Subject)
			assert.Equal(t, tt.from, msg.From)
			assert.Equal(t, []string{"bob@example.com"}, msg.To)
			assert.Contains(t, msg.BodyText, tt.body)
			// The headers map carries the full decoded header set.
			assert.Equal(t, []string{tt.subject}, msg.Headers["Subject"])
			assert.Equal(t, []string{tt.from}, msg.Headers["From"])
			assert.Equal(t, []string{"bob@example.com"}, msg.Headers["To"])
		})
	}
}

func TestGetMessage_Attachment(t *testing.T) {
	svc := newSeededIMAP(t)

	msg, err := svc.GetMessage(t.Context(), "INBOX", 2)
	require.NoError(t, err)
	require.Len(t, msg.Attachments, 1)
	att := msg.Attachments[0]
	assert.Equal(t, "report.pdf", att.Filename)
	assert.Equal(t, "application/pdf", att.ContentType)
	assert.Equal(t, int64(len(attachmentBytes)), att.Size)
}

func TestGetMessage_UnknownUID(t *testing.T) {
	svc := newSeededIMAP(t)

	_, err := svc.GetMessage(t.Context(), "INBOX", 99)
	assert.Error(t, err)
}

func TestNewMailService_BadPassword(t *testing.T) {
	host, port, roots := emailtest.StartIMAP(t, testIMAPUser, testIMAPPassword, map[string][]emailtest.SeedMessage{
		"INBOX": {},
	})
	stubTLSRoots(t, roots)

	_, err := newMailService(&credentials{
		Username: testIMAPUser,
		Password: "wrong-" + testIMAPPassword,
		IMAP:     serverConfig{Host: host, Port: port},
	})
	require.Error(t, err)
	// The password must never surface in an error string, even a wrong one.
	assert.NotContains(t, err.Error(), "wrong-"+testIMAPPassword)
}
