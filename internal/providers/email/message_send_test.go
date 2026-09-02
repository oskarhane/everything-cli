package email

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/config"
)

// sendFakeService is a MailService fake narrowed to the send concern: it
// captures the SendInput (body eagerly drained, since the reader is
// single-use) and records whether Close ran. The other MailService methods
// only satisfy the interface; the send leaf never calls them.
type sendFakeService struct {
	in      SendInput
	body    string
	sendErr error
	closed  bool
}

func (s *sendFakeService) SendMessage(_ context.Context, in SendInput) error {
	s.in = in
	if in.Body != nil {
		content, err := io.ReadAll(in.Body)
		if err == nil {
			s.body = string(content)
		}
	}
	return s.sendErr
}

func (s *sendFakeService) ListMailboxes(context.Context) ([]string, error) { return nil, nil }
func (s *sendFakeService) ListEnvelopes(context.Context, string, int) ([]Envelope, error) {
	return nil, nil
}
func (s *sendFakeService) GetMessage(context.Context, string, uint32) (*Message, error) {
	return nil, nil
}
func (s *sendFakeService) Close() error { s.closed = true; return nil }

// stubSendDial swaps the dialMail seam so the send leaf returns the fake
// service instead of touching the network.
func stubSendDial(t *testing.T, svc MailService, err error) {
	t.Helper()
	saved := dialMail
	dialMail = func(context.Context, *app.Config) (MailService, error) { return svc, err }
	t.Cleanup(func() { dialMail = saved })
}

// newSendEnv returns a hermetic tree mounting only the send leaf
// (email → message → send), independent of sibling-owned parent files. The
// config FS is in-memory and the config dir is pinned, mirroring newEmailEnv.
func newSendEnv(t *testing.T) (*app.Config, *cobra.Command, *bytes.Buffer) {
	t.Helper()
	t.Setenv(config.EnvConfigDir, "/config")
	t.Setenv(passwordEnvVar, "")
	cfg := &app.Config{Fs: afero.NewMemMapFs()}
	root := &cobra.Command{Use: "everything-cli", SilenceErrors: true, SilenceUsage: true}
	msg := &cobra.Command{Use: "message"}
	msg.AddCommand(newMessageSendCmd(cfg))
	emailCmd := &cobra.Command{Use: "email"}
	emailCmd.AddCommand(msg)
	root.AddCommand(emailCmd)
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(io.Discard)
	return cfg, root, out
}

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
			cfg, root, out := newSendEnv(t)
			if tt.setup != nil {
				tt.setup(t, cfg, root)
			}
			svc := &sendFakeService{}
			stubSendDial(t, svc, nil)

			got, err := execute(t, root, out, append([]string{"email", "message", "send"}, tt.args...)...)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Empty(t, svc.body, "rejected input must not reach the service")
				assert.False(t, svc.closed, "usage errors must not dial")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantTo, svc.in.To)
			assert.Equal(t, tt.wantCc, svc.in.Cc)
			assert.Equal(t, tt.wantSubject, svc.in.Subject)
			assert.Equal(t, tt.wantBody, svc.body)
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
	cfg, root, out := newSendEnv(t)
	const password = "send-leaf-distinctive-password-9f2"
	seedAccount(t, cfg, "work", password)
	svc := &sendFakeService{}
	stubSendDial(t, svc, nil)

	got, err := execute(t, root, out,
		"email", "message", "send",
		"--to", "alice@example.com", "--to", "bob@example.com",
		"--subject", "s", "--body", "b")
	require.NoError(t, err)

	assert.JSONEq(t, `{"sent": true, "to": ["alice@example.com", "bob@example.com"]}`,
		strings.TrimSpace(got))
	assert.NotContains(t, got, password)
}

func TestMessageSendPropagatesSendError(t *testing.T) {
	_, root, out := newSendEnv(t)
	svc := &sendFakeService{sendErr: errors.New("smtp: 550 mailbox unavailable")}
	stubSendDial(t, svc, nil)

	_, err := execute(t, root, out,
		"email", "message", "send",
		"--to", "alice@example.com", "--subject", "s", "--body", "b")

	require.ErrorContains(t, err, "smtp: 550 mailbox unavailable")
	assert.True(t, svc.closed, "the service is closed even when the send fails")
}

func TestMessageSendPropagatesDialError(t *testing.T) {
	_, root, out := newSendEnv(t)
	stubSendDial(t, nil, errors.New("no account configured"))

	_, err := execute(t, root, out,
		"email", "message", "send",
		"--to", "alice@example.com", "--subject", "s", "--body", "b")

	require.ErrorContains(t, err, "no account configured")
}
