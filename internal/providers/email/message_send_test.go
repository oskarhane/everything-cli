package email

import (
	"errors"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/emailtest"
)

func TestMessageSend(t *testing.T) {
	tests := []struct {
		name string
		// setup seeds files/stdin before execution.
		setup func(t *testing.T, cfg *app.Config, root *cobra.Command)
		args  []string
		// want* describe the captured SendInput; empty wantErr means success.
		wantTo      []string
		wantCc      []string
		wantSubject string
		wantBody    string
		wantErr     string
	}{
		{
			name:        "inline body",
			args:        []string{"--to", "alice@example.com", "--subject", "Lunch", "--body", "Noon works"},
			wantTo:      []string{"alice@example.com"},
			wantSubject: "Lunch",
			wantBody:    "Noon works",
		},
		{
			name: "repeatable to and cc",
			args: []string{
				"--to", "a@example.com", "--to", "b@example.com",
				"--cc", "carol@example.com", "--cc", "dave@example.com",
				"--subject", "s", "--body", "b",
			},
			wantTo:      []string{"a@example.com", "b@example.com"},
			wantCc:      []string{"carol@example.com", "dave@example.com"},
			wantSubject: "s",
			wantBody:    "b",
		},
		{
			name: "body from file",
			setup: func(t *testing.T, cfg *app.Config, _ *cobra.Command) {
				require.NoError(t, afero.WriteFile(cfg.Fs, "note.txt", []byte("file body"), 0o644))
			},
			args:        []string{"--to", "alice@example.com", "--subject", "s", "--body-file", "note.txt"},
			wantTo:      []string{"alice@example.com"},
			wantSubject: "s",
			wantBody:    "file body",
		},
		{
			name: "body from stdin",
			setup: func(_ *testing.T, _ *app.Config, root *cobra.Command) {
				root.SetIn(strings.NewReader("piped body"))
			},
			args:        []string{"--to", "alice@example.com", "--subject", "s", "--body-file", "-"},
			wantTo:      []string{"alice@example.com"},
			wantSubject: "s",
			wantBody:    "piped body",
		},
		{
			name:    "no body source is a usage error",
			args:    []string{"--to", "alice@example.com", "--subject", "s"},
			wantErr: "no message body: pass exactly one of --body or --body-file",
		},
		{
			name: "both body sources is a usage error",
			args: []string{
				"--to", "alice@example.com", "--subject", "s",
				"--body", "inline", "--body-file", "note.txt",
			},
			wantErr: "--body and --body-file are mutually exclusive",
		},
		{
			name:    "missing to is a usage error",
			args:    []string{"--subject", "s", "--body", "b"},
			wantErr: "no recipients",
		},
		{
			name:    "missing subject is a usage error",
			args:    []string{"--to", "alice@example.com", "--body", "b"},
			wantErr: "no subject",
		},
		{
			name:    "positional args rejected",
			args:    []string{"extra", "--to", "alice@example.com", "--subject", "s", "--body", "b"},
			wantErr: "unknown command",
		},
		{
			name:    "missing body file",
			args:    []string{"--to", "alice@example.com", "--subject", "s", "--body-file", "nope.txt"},
			wantErr: "reading --body-file",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, root, out := newEmailEnv(t)
			if tt.setup != nil {
				tt.setup(t, cfg, root)
			}
			svc := sendFake(nil)
			stubDial(t, &dialSendMail, svc, nil)

			got, err := execute(t, root, out, append([]string{"email", "message", "send"}, tt.args...)...)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Empty(t, svc.gotBody, "rejected input must not reach the service")
				assert.False(t, svc.closed, "usage errors must not dial")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantTo, svc.gotSend.To)
			assert.Equal(t, tt.wantCc, svc.gotSend.Cc)
			assert.Equal(t, tt.wantSubject, svc.gotSend.Subject)
			assert.Equal(t, tt.wantBody, svc.gotBody)
			assert.True(t, svc.closed, "the leaf must close the service after a successful dial")
			assert.Contains(t, got, `"sent": true`)
		})
	}
}

// TestMessageSendConfirmationShape pins the success output: exactly the
// {"sent": true, "to": [...]} object with snake_case keys, and never the
// account password — even though the account is seeded with a distinctive
// one, output must carry nothing credential-shaped.
func TestMessageSendConfirmationShape(t *testing.T) {
	cfg, root, out := newEmailEnv(t)
	const password = "send-leaf-distinctive-password-9f2"
	seedAccount(t, cfg, "work", password)
	svc := sendFake(nil)
	stubDial(t, &dialSendMail, svc, nil)

	got, err := execute(t, root, out,
		"email", "message", "send",
		"--to", "alice@example.com", "--to", "bob@example.com",
		"--subject", "s", "--body", "b")
	require.NoError(t, err)

	assert.JSONEq(t, `{"sent": true, "to": ["alice@example.com", "bob@example.com"]}`,
		strings.TrimSpace(got))
	assert.NotContains(t, got, password)
}

// TestMessageSendDialsSMTPOnly runs the send leaf against the REAL
// dialSendMail seam (no stub) with an account whose IMAP endpoint refuses
// connections and whose SMTP endpoint is the in-process test server. The
// send must succeed without any IMAP dial or login being attempted —
// before this fix the leaf dialed IMAP first and the command failed.
func TestMessageSendDialsSMTPOnly(t *testing.T) {
	server := emailtest.StartSMTP(t, testSMTPUser, testSMTPPassword, false)
	stubTLSRoots(t, server.Roots)

	cfg, root, out := newEmailEnv(t)
	// Seed directly (seedAccount pins unreachable hosts): IMAP points at a
	// refused loopback port, so any IMAP dial or login attempt fails.
	_, err := addAccount(newStore(t, cfg), addOptions{
		Name:     "work",
		Username: testSMTPUser,
		Password: testSMTPPassword,
		IMAPHost: "127.0.0.1",
		IMAPPort: 1, // nothing listens: refused
		SMTPHost: server.Host,
		SMTPPort: server.Port,
	})
	require.NoError(t, err)

	got, err := execute(t, root, out,
		"email", "message", "send",
		"--to", "alice@example.com", "--subject", "s", "--body", "b")
	require.NoError(t, err, "send must not depend on IMAP reachability")
	assert.Contains(t, got, `"sent": true`)

	msgs := server.Messages()
	require.Len(t, msgs, 1, "the message reached the SMTP server")
	assert.Contains(t, string(msgs[0].Data), "Subject: s")
}

func TestMessageSendPropagatesSendError(t *testing.T) {
	_, root, out := newEmailEnv(t)
	svc := sendFake(errors.New("smtp: 550 mailbox unavailable"))
	stubDial(t, &dialSendMail, svc, nil)

	_, err := execute(t, root, out,
		"email", "message", "send",
		"--to", "alice@example.com", "--subject", "s", "--body", "b")

	require.ErrorContains(t, err, "smtp: 550 mailbox unavailable")
	assert.True(t, svc.closed, "the service is closed even when the send fails")
}

func TestMessageSendPropagatesDialError(t *testing.T) {
	_, root, out := newEmailEnv(t)
	stubDial(t, &dialSendMail, nil, errors.New("no account configured"))

	_, err := execute(t, root, out,
		"email", "message", "send",
		"--to", "alice@example.com", "--subject", "s", "--body", "b")

	require.ErrorContains(t, err, "no account configured")
}
