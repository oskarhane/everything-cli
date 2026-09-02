package email

import (
	"crypto/tls"
	"crypto/x509"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/providers/emailtest"
)

const (
	testSMTPUser     = "sender@example.com"
	testSMTPPassword = "send-test-secret"
)

// stubSMTPTLSRoots points the adapter's TLS config seam at the test CA.
// (adapter_test.go has the IMAP twin; both swap the same package seam.)
func stubSMTPTLSRoots(t *testing.T, roots *x509.CertPool) {
	t.Helper()
	saved := tlsConfigFor
	tlsConfigFor = func(host string) *tls.Config {
		cfg := saved(host)
		cfg.RootCAs = roots
		return cfg
	}
	t.Cleanup(func() { tlsConfigFor = saved })
}

// TestSendMessage proves the adapter submits a composed RFC 5322 message
// over both TLS transports: STARTTLS (submission, 587) and implicit TLS
// (submissions, 465), with the loopback port standing in for each.
func TestSendMessage(t *testing.T) {
	tests := []struct {
		name        string
		implicitTLS bool
	}{
		{name: "starttls", implicitTLS: false},
		{name: "implicit tls", implicitTLS: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := emailtest.StartSMTP(t, testSMTPUser, testSMTPPassword, tt.implicitTLS)
			stubSMTPTLSRoots(t, server.Roots)
			if tt.implicitTLS {
				// Pin the port→transport mapping onto the loopback port
				// (production maps exactly 465 to implicit TLS).
				saved := smtpUsesImplicitTLS
				smtpUsesImplicitTLS = func(port int) bool { return port == server.Port }
				t.Cleanup(func() { smtpUsesImplicitTLS = saved })
			}

			svc := &mailService{creds: &credentials{
				Username: testSMTPUser,
				Password: testSMTPPassword,
				SMTP:     serverConfig{Host: server.Host, Port: server.Port},
			}}
			err := svc.SendMessage(t.Context(), SendInput{
				To:      []string{"Bob <bob@example.com>"},
				Cc:      []string{"carol@example.com"},
				Subject: "Adapter send test",
				Body:    strings.NewReader("hello from the adapter test"),
			})
			require.NoError(t, err)

			msgs := server.Messages()
			require.Len(t, msgs, 1)
			got := msgs[0]
			assert.Equal(t, testSMTPUser, got.From)
			assert.ElementsMatch(t, []string{"bob@example.com", "carol@example.com"}, got.To)

			data := string(got.Data)
			// Header values take net/mail's canonical form (display names
			// are quoted, bare addresses bracketed).
			assert.Contains(t, data, `To: "Bob" <bob@example.com>`)
			assert.Contains(t, data, "Cc: <carol@example.com>")
			assert.Contains(t, data, "Subject: Adapter send test")
			assert.Contains(t, data, "hello from the adapter test")
			// The password is an AUTH exchange, never message content.
			assert.NotContains(t, data, testSMTPPassword)
		})
	}
}

func TestSendMessage_NoRecipient(t *testing.T) {
	svc := &mailService{creds: &credentials{
		Username: testSMTPUser,
		Password: testSMTPPassword,
		SMTP:     serverConfig{Host: "127.0.0.1", Port: 1},
	}}
	err := svc.SendMessage(t.Context(), SendInput{Subject: "nobody home"})
	assert.Error(t, err)
}
