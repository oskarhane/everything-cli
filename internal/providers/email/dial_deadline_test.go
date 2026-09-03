package email

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/providers/emailtest"
)

// stubDialSetupTimeout shrinks the connect+setup budget for the test's
// lifetime — same seam style as stubTLSRoots/stubIMAPImplicitTLS.
func stubDialSetupTimeout(t *testing.T, d time.Duration) {
	t.Helper()
	saved := dialSetupTimeout
	dialSetupTimeout = d
	t.Cleanup(func() { dialSetupTimeout = saved })
}

// loopbackPort extracts the port from a 127.0.0.1 listener.
func loopbackPort(t *testing.T, ln net.Listener) int {
	t.Helper()
	port, err := strconv.Atoi(strings.Split(ln.Addr().String(), ":")[1])
	require.NoError(t, err)
	return port
}

// TestSendMessage_SilentSMTPServerTimesOut reproduces the reported hang:
// a server that accepts the TCP connection but NEVER sends the SMTP
// greeting. `email message send` must fail within the (shrunk) setup
// budget with an error naming the server and the timeout — not hang.
func TestSendMessage_SilentSMTPServerTimesOut(t *testing.T) {
	stubDialSetupTimeout(t, 300*time.Millisecond)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		// Accept and stay silent: never a greeting, never a byte. Drain
		// whatever the client sends until it gives up and hangs up.
		_, _ = io.Copy(io.Discard, conn)
		_ = conn.Close()
	}()
	port := loopbackPort(t, ln)

	svc := &mailService{creds: &credentials{
		Username: testSMTPUser,
		Password: testSMTPPassword,
		SMTP:     serverConfig{Host: "127.0.0.1", Port: port},
	}}
	start := time.Now()
	err = svc.SendMessage(t.Context(), SendInput{
		To:      []string{"bob@example.com"},
		Subject: "silent server",
		Body:    strings.NewReader("hello"),
	})
	elapsed := time.Since(start)

	require.Error(t, err, "a silent server must fail the send, not hang")
	assert.Contains(t, err.Error(), fmt.Sprintf("smtp server 127.0.0.1:%d", port))
	assert.Contains(t, err.Error(), "timed out")
	assert.NotContains(t, err.Error(), testSMTPPassword)
	assert.Less(t, elapsed, 5*time.Second, "bounded by the shrunk setup budget, not a hang")
}

// TestDialIMAPStartTLS_StalledHandshakeTimesOut proves the IMAP STARTTLS
// setup window is bounded: the server answers the greeting and OK to
// STARTTLS, then stalls the TLS handshake (never speaks TLS).
// imapclient's NewStartTLS runs that Handshake with no ctx and no
// deadline of its own, so without the setup clamp this dial hangs
// forever.
func TestDialIMAPStartTLS_StalledHandshakeTimesOut(t *testing.T) {
	stubDialSetupTimeout(t, 300*time.Millisecond)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		_, _ = io.WriteString(conn, "* OK test imap server\r\n")
		sc := bufio.NewScanner(conn)
		for sc.Scan() {
			line := sc.Text()
			if strings.HasSuffix(line, "STARTTLS") {
				_, _ = fmt.Fprintf(conn, "%s OK begin TLS negotiation\r\n", strings.Fields(line)[0])
				// Then stall: drain the ClientHello, never answer it.
				_, _ = io.Copy(io.Discard, conn)
				return
			}
		}
	}()
	port := loopbackPort(t, ln)

	start := time.Now()
	_, err = dialIMAPTLS(t.Context(), serverConfig{Host: "127.0.0.1", Port: port})
	elapsed := time.Since(start)

	require.Error(t, err, "a stalled TLS handshake must fail the dial, not hang")
	assert.Contains(t, err.Error(), fmt.Sprintf("imap server 127.0.0.1:%d", port))
	assert.Contains(t, err.Error(), "timed out")
	assert.Less(t, elapsed, 5*time.Second, "bounded by the shrunk setup budget, not a hang")
}

// TestDialIMAPStartTLS_DeadlineClearedAfterSetup proves the setup
// deadline is lifted once the handshake completes: a connection that
// outlives the (shrunk) setup budget must still serve long-lived reads.
func TestDialIMAPStartTLS_DeadlineClearedAfterSetup(t *testing.T) {
	stubDialSetupTimeout(t, 300*time.Millisecond)

	host, port, roots := emailtest.StartIMAPStartTLS(t, testIMAPUser, testIMAPPassword,
		map[string][]emailtest.SeedMessage{"INBOX": {}})
	stubTLSRoots(t, roots)

	svc, err := newMailService(t.Context(), &credentials{
		Username: testIMAPUser,
		Password: testIMAPPassword,
		IMAP:     serverConfig{Host: host, Port: port},
	})
	require.NoError(t, err, "setup within the budget still succeeds")
	t.Cleanup(func() { _ = svc.Close() })

	time.Sleep(500 * time.Millisecond) // outlive the armed setup budget
	names, err := svc.ListMailboxes(t.Context())
	require.NoError(t, err, "reads after the setup budget must not see the cleared deadline")
	assert.Contains(t, names, "INBOX")
}
