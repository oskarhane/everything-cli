package email

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/config"
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

// startIMAP starts an in-process IMAP server seeded with seed and stubs
// the TLS roots so the adapter verifies against the test CA. The port→
// transport mapping is pinned onto the loopback port so the dial takes
// the implicit-TLS path the server speaks.
func startIMAP(t *testing.T, seed map[string][]emailtest.SeedMessage) serverConfig {
	t.Helper()
	host, port, roots := emailtest.StartIMAP(t, testIMAPUser, testIMAPPassword, seed)
	stubTLSRoots(t, roots)
	stubIMAPImplicitTLS(t, port)
	return serverConfig{Host: host, Port: port}
}

// connectIMAP starts a seeded in-process IMAP server, connects an adapter
// to it with the test credentials, and closes it at test cleanup.
func connectIMAP(t *testing.T, seed map[string][]emailtest.SeedMessage) *mailService {
	t.Helper()
	svc, err := newMailService(t.Context(), &credentials{
		Username: testIMAPUser,
		Password: testIMAPPassword,
		IMAP:     startIMAP(t, seed),
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })
	return svc
}

// newSeededIMAP connects an adapter to an in-process IMAP server with
// INBOX (one simple and one multipart message, the first flagged \Seen)
// and Archive (empty).
func newSeededIMAP(t *testing.T) *mailService {
	t.Helper()
	return connectIMAP(t, map[string][]emailtest.SeedMessage{
		"INBOX": {
			{Raw: simpleRawMessage(), Flags: []string{`\Seen`}, Time: testSimpleDate},
			{Raw: multipartRawMessage(), Time: testMultipartDate},
		},
		"Archive": {},
	})
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
	// Size comes from BODYSTRUCTURE metadata (the attachment body is never
	// fetched), so it is the server-declared encoded octet count: the raw
	// message carries the payload base64-encoded.
	encodedLen := int64(len(base64.StdEncoding.EncodeToString(attachmentBytes)))
	assert.Equal(t, encodedLen, att.Size)
	assert.NotEqual(t, int64(len(attachmentBytes)), att.Size,
		"BODYSTRUCTURE size counts the encoded octets, not the decoded bytes")
}

// oversizedRawMessage builds a multipart message whose attachment is an
// 8 MiB base64 payload (11 MiB on the wire): big enough that buffering it
// to compute Size would be wasteful, and catastrophic at hostile sizes.
func oversizedRawMessage() string {
	big := make([]byte, 8<<20)
	for i := range big {
		big[i] = byte(i)
	}
	return "MIME-Version: 1.0\r\n" +
		"From: Alice <alice@example.com>\r\n" +
		"To: bob@example.com\r\n" +
		"Subject: Big attachment\r\n" +
		"Date: " + testSimpleDate.Format(time.RFC1123Z) + "\r\n" +
		"Content-Type: multipart/mixed; boundary=\"big-boundary\"\r\n" +
		"\r\n" +
		"--big-boundary\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"\r\n" +
		"Small body, huge attachment.\r\n" +
		"--big-boundary\r\n" +
		"Content-Type: application/octet-stream; name=\"big.bin\"\r\n" +
		"Content-Disposition: attachment; filename=\"big.bin\"\r\n" +
		"Content-Transfer-Encoding: base64\r\n" +
		"\r\n" +
		base64.StdEncoding.EncodeToString(big) + "\r\n" +
		"--big-boundary--\r\n"
}

// TestGetMessage_OversizedAttachmentNotBuffered proves the adapter derives
// attachment metadata from BODYSTRUCTURE and never transfers (let alone
// buffers) the attachment body: the result carries the server-declared
// size and the small text body, and the fetch of an 11 MiB message returns
// promptly because only the header and the text part cross the wire.
func TestGetMessage_OversizedAttachmentNotBuffered(t *testing.T) {
	raw := oversizedRawMessage()
	svc := connectIMAP(t, map[string][]emailtest.SeedMessage{
		"INBOX": {{Raw: raw, Time: testSimpleDate}},
	})

	start := time.Now()
	msg, err := svc.GetMessage(t.Context(), "INBOX", 1)
	elapsed := time.Since(start)
	require.NoError(t, err)

	assert.Contains(t, msg.BodyText, "Small body, huge attachment.")
	require.Len(t, msg.Attachments, 1)
	att := msg.Attachments[0]
	assert.Equal(t, "big.bin", att.Filename)
	assert.Equal(t, "application/octet-stream", att.ContentType)
	// The size is the BODYSTRUCTURE-encoded octet count: exact metadata
	// without a single attachment byte in process memory.
	wantSize := int64(len(base64.StdEncoding.EncodeToString(make([]byte, 8<<20))))
	assert.Equal(t, wantSize, att.Size)
	assert.Less(t, elapsed, 10*time.Second,
		"the attachment body must not be fetched or buffered")
}

// TestGetMessage_AttachmentBodyNeverRead pins the same guarantee
// deterministically: the fixture's attachment declares base64 but carries
// invalid base64. Any code path that read and decoded the attachment body
// would fail; the BODYSTRUCTURE path never touches it.
func TestGetMessage_AttachmentBodyNeverRead(t *testing.T) {
	raw := "MIME-Version: 1.0\r\n" +
		"From: Alice <alice@example.com>\r\n" +
		"To: bob@example.com\r\n" +
		"Subject: Corrupt attachment\r\n" +
		"Date: " + testSimpleDate.Format(time.RFC1123Z) + "\r\n" +
		"Content-Type: multipart/mixed; boundary=\"corrupt-boundary\"\r\n" +
		"\r\n" +
		"--corrupt-boundary\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"\r\n" +
		"Body is fine.\r\n" +
		"--corrupt-boundary\r\n" +
		"Content-Type: application/pdf; name=\"broken.pdf\"\r\n" +
		"Content-Disposition: attachment; filename=\"broken.pdf\"\r\n" +
		"Content-Transfer-Encoding: base64\r\n" +
		"\r\n" +
		"%%%not-valid-base64!!!%%%\r\n" +
		"--corrupt-boundary--\r\n"
	svc := connectIMAP(t, map[string][]emailtest.SeedMessage{
		"INBOX": {{Raw: raw, Time: testSimpleDate}},
	})

	msg, err := svc.GetMessage(t.Context(), "INBOX", 1)
	require.NoError(t, err, "the corrupt attachment body is never read")
	assert.Contains(t, msg.BodyText, "Body is fine.")
	require.Len(t, msg.Attachments, 1)
	assert.Equal(t, "broken.pdf", msg.Attachments[0].Filename)
	assert.Equal(t, int64(len("%%%not-valid-base64!!!%%%")), msg.Attachments[0].Size)
}

func TestGetMessage_UnknownUID(t *testing.T) {
	svc := newSeededIMAP(t)

	_, err := svc.GetMessage(t.Context(), "INBOX", 99)
	assert.Error(t, err)
}

func TestNewMailService_BadPassword(t *testing.T) {
	_, err := newMailService(t.Context(), &credentials{
		Username: testIMAPUser,
		Password: "wrong-" + testIMAPPassword,
		IMAP:     startIMAP(t, map[string][]emailtest.SeedMessage{"INBOX": {}}),
	})
	require.Error(t, err)
	// The password must never surface in an error string, even a wrong one.
	assert.NotContains(t, err.Error(), "wrong-"+testIMAPPassword)
}

// seedLegacyAccount persists an account in the PRE-FIX shape: the port is
// embedded in the stored host while the stored port field holds the old
// default. Accounts added before host:port normalization look like this.
func seedLegacyAccount(t *testing.T, cfg *app.Config, name string, imapHost string, imapPort int) {
	t.Helper()
	payload, err := json.Marshal(credentials{
		Username: testIMAPUser,
		Password: testIMAPPassword,
		IMAP:     serverConfig{Host: imapHost, Port: imapPort},
		SMTP:     serverConfig{Host: "smtp.example.com", Port: defaultSMTPPort},
	})
	require.NoError(t, err)
	err = newStore(t, cfg).Save(&config.Account{
		Name:     name,
		Provider: providerID,
		Auth:     payload,
	})
	require.NoError(t, err)
}

// TestMailboxListLegacyEmbeddedPort proves a legacy account — stored
// before the add-time fix, host "127.0.0.1:<port>" with the port field at
// the 993 default — still dials the embedded port: the full read path
// (real dialMail seam, loopback IMAP server on a random port) succeeds
// only if the dial targets the embedded port, not 993.
func TestMailboxListLegacyEmbeddedPort(t *testing.T) {
	cfg, root, out := newEmailEnv(t)
	host, port, roots := emailtest.StartIMAP(t, testIMAPUser, testIMAPPassword,
		map[string][]emailtest.SeedMessage{"INBOX": {}})
	stubTLSRoots(t, roots)
	stubIMAPImplicitTLS(t, port)
	seedLegacyAccount(t, cfg, "legacy",
		host+":"+strconv.Itoa(port), defaultIMAPPort)

	stdout, err := execute(t, root, out,
		"email", "mailbox", "list", "--account", "legacy", "--format", "json")
	require.NoError(t, err, "the dial must target the embedded port, not the stored 993")
	assert.Contains(t, stdout, "INBOX")
}

// TestDialErrorRendersResolvedHostPort pins the dial-failure message: the
// resolved host:port renders cleanly, never the buggy "[host:port]:port"
// form a port-embedded host produced before the fix.
func TestDialErrorRendersResolvedHostPort(t *testing.T) {
	// Legacy shape, nothing listening on 127.0.0.1:1.
	_, err := newMailService(t.Context(), &credentials{
		Username: testIMAPUser,
		Password: testIMAPPassword,
		IMAP:     serverConfig{Host: "127.0.0.1:1", Port: defaultIMAPPort},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "connecting to imap server 127.0.0.1:1:")
	assert.NotContains(t, err.Error(), "[127.0.0.1:1]")
}

// TestIMAPUseImplicitTLS pins the IMAP port→transport mapping, the
// sibling of the SMTP 465/587 rule: 993 is implicit TLS, every other
// port is mandatory STARTTLS.
func TestIMAPUseImplicitTLS(t *testing.T) {
	assert.True(t, imapUsesImplicitTLS(993))
	assert.False(t, imapUsesImplicitTLS(143))
	assert.False(t, imapUsesImplicitTLS(1143))
}

// TestDialIMAPStartTLS proves the non-993 path end to end: the server
// listens in PLAINTEXT and only upgrades on STARTTLS, so a successful
// login and mailbox list can only have happened over a connection that
// started cleartext and upgraded. Certificate verification is full —
// the loopback CA goes through the RootCAs seam, nothing skipped.
func TestDialIMAPStartTLS(t *testing.T) {
	host, port, roots := emailtest.StartIMAPStartTLS(t, testIMAPUser, testIMAPPassword,
		map[string][]emailtest.SeedMessage{
			"INBOX":   {{Raw: simpleRawMessage(), Flags: []string{`\Seen`}, Time: testSimpleDate}},
			"Archive": {},
		})
	stubTLSRoots(t, roots)

	svc, err := newMailService(t.Context(), &credentials{
		Username: testIMAPUser,
		Password: testIMAPPassword,
		IMAP:     serverConfig{Host: host, Port: port},
	})
	require.NoError(t, err, "login over the STARTTLS-upgraded connection")
	t.Cleanup(func() { _ = svc.Close() })

	names, err := svc.ListMailboxes(t.Context())
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"INBOX", "Archive"}, names)
}

// TestDialIMAPExplicitStartTLSOverridesHeuristic proves the explicit
// transport wins over the port heuristic: the stored account carries
// imap.tls == "starttls" on a port the heuristic maps to IMPLICIT TLS
// (the seam pins the loopback port as if it were 993), yet the dial must
// still take the STARTTLS path — the server listens in plaintext and
// only upgrades on STARTTLS, so a successful login and list can only
// have happened over an upgraded connection.
func TestDialIMAPExplicitStartTLSOverridesHeuristic(t *testing.T) {
	host, port, roots := emailtest.StartIMAPStartTLS(t, testIMAPUser, testIMAPPassword,
		map[string][]emailtest.SeedMessage{
			"INBOX": {{Raw: simpleRawMessage(), Flags: []string{`\Seen`}, Time: testSimpleDate}},
		})
	stubTLSRoots(t, roots)
	stubIMAPImplicitTLS(t, port) // heuristic says implicit, as on 993

	svc, err := newMailService(t.Context(), &credentials{
		Username: testIMAPUser,
		Password: testIMAPPassword,
		IMAP:     serverConfig{Host: host, Port: port, TLS: tlsModeStartTLS},
	})
	require.NoError(t, err, "explicit starttls must override the implicit-TLS heuristic")
	t.Cleanup(func() { _ = svc.Close() })

	names, err := svc.ListMailboxes(t.Context())
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"INBOX"}, names)
}

// TestDialIMAPStartTLS_Mandatory proves STARTTLS is mandatory on the
// non-993 path: a server that refuses STARTTLS fails the dial, and the
// only command that ever crossed the plaintext connection is STARTTLS
// itself — no IMAP command (LOGIN, LIST, ...) runs unencrypted. The
// server is a raw loopback stub (no TLSConfig at all) that records
// every command line it receives and answers NO to STARTTLS.
func TestDialIMAPStartTLS_Mandatory(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })

	var (
		mu       sync.Mutex
		received []string
	)
	hungUp := make(chan struct{})
	go func() {
		defer close(hungUp)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		_, _ = io.WriteString(conn, "* OK test server\r\n")
		sc := bufio.NewScanner(conn)
		for sc.Scan() {
			line := sc.Text()
			mu.Lock()
			received = append(received, line)
			mu.Unlock()
			if strings.HasSuffix(line, "STARTTLS") {
				_, _ = fmt.Fprintf(conn, "%s NO STARTTLS not supported\r\n", strings.Fields(line)[0])
			}
		}
	}()

	port, err := strconv.Atoi(strings.Split(ln.Addr().String(), ":")[1])
	require.NoError(t, err)
	_, err = dialIMAPTLS(t.Context(), serverConfig{Host: "127.0.0.1", Port: port})
	require.Error(t, err, "a server without STARTTLS must fail the dial")
	assert.Contains(t, err.Error(), "starttls with imap server")

	// The refusal makes the client hang up; every line it ever sent is
	// then recorded.
	<-hungUp
	mu.Lock()
	defer mu.Unlock()
	require.Len(t, received, 1, "only STARTTLS may cross the plaintext connection")
	assert.True(t, strings.HasSuffix(received[0], "STARTTLS"), "received: %q", received[0])
}
