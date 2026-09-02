package email

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/app"
)

// fakeMailboxService is a MailService whose only real behavior is
// ListMailboxes; the other concern methods error out, so a call through any
// of them fails the test and proves the leaf consumes only the narrowed
// MailboxLister surface.
type fakeMailboxService struct {
	names   []string
	listErr error
	closed  bool
}

func (f *fakeMailboxService) ListMailboxes(context.Context) ([]string, error) {
	return f.names, f.listErr
}

func (f *fakeMailboxService) ListEnvelopes(context.Context, string, int) ([]Envelope, error) {
	return nil, errors.New("unexpected ListEnvelopes call")
}

func (f *fakeMailboxService) GetMessage(context.Context, string, uint32) (*Message, error) {
	return nil, errors.New("unexpected GetMessage call")
}

func (f *fakeMailboxService) SendMessage(context.Context, SendInput) error {
	return errors.New("unexpected SendMessage call")
}

func (f *fakeMailboxService) Close() error {
	f.closed = true
	return nil
}

// stubDialMail swaps the dial seam so the leaf resolves svc instead of
// touching the network; restored via t.Cleanup.
func stubDialMail(t *testing.T, svc MailService, err error) {
	t.Helper()
	saved := dialMail
	dialMail = func(context.Context, *app.Config) (MailService, error) {
		return svc, err
	}
	t.Cleanup(func() { dialMail = saved })
}

// newMailboxEnv returns the shared hermetic env with the mailbox subtree
// mounted at its production path. Wiring into provider.go is a separate
// change, so the tests attach newMailboxCmd onto the registered provider
// command themselves.
func newMailboxEnv(t *testing.T) (*cobra.Command, *bytes.Buffer) {
	t.Helper()
	cfg, root, out := newEmailEnv(t)
	emailCmd, _, err := root.Find([]string{"email"})
	require.NoError(t, err)
	emailCmd.AddCommand(newMailboxCmd(cfg))
	return root, out
}

func TestMailboxList(t *testing.T) {
	tests := []struct {
		name      string
		format    string
		names     []string
		wantIn    []string
		wantNotIn []string
	}{
		{
			name:   "json array of name rows",
			format: "json",
			names:  []string{"Archive", "INBOX", "Sent"},
			wantIn: []string{`"name": "Archive"`, `"name": "INBOX"`, `"name": "Sent"`},
		},
		{
			name:   "table upper-case header",
			format: "table",
			names:  []string{"INBOX", "Sent"},
			// go-pretty StyleLight upper-cases header cells (AGENTS.md).
			wantIn: []string{"NAME", "INBOX", "Sent"},
		},
		{
			name:   "toon rows",
			format: "toon",
			names:  []string{"INBOX", "Sent"},
			wantIn: []string{"name", "INBOX", "Sent"},
		},
		{
			name:      "empty list renders empty json array",
			format:    "json",
			names:     nil,
			wantIn:    []string{"[]"},
			wantNotIn: []string{"name"},
		},
		{
			name:      "single mailbox collapses to a json object",
			format:    "json",
			names:     []string{"INBOX"},
			wantIn:    []string{`"name": "INBOX"`},
			wantNotIn: []string{"["},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeMailboxService{names: tt.names}
			stubDialMail(t, svc, nil)
			root, out := newMailboxEnv(t)

			stdout, err := execute(t, root, out, "email", "mailbox", "list", "--format", tt.format)
			require.NoError(t, err)
			for _, want := range tt.wantIn {
				assert.Contains(t, stdout, want)
			}
			for _, unwanted := range tt.wantNotIn {
				assert.NotContains(t, stdout, unwanted)
			}
			assert.True(t, svc.closed, "leaf must close the service after a successful dial")
		})
	}
}

func TestMailboxListErrors(t *testing.T) {
	t.Run("dial failure propagates", func(t *testing.T) {
		stubDialMail(t, nil, errors.New("dial boom"))
		root, out := newMailboxEnv(t)
		_, err := execute(t, root, out, "email", "mailbox", "list")
		require.ErrorContains(t, err, "dial boom")
	})

	t.Run("list failure propagates and still closes", func(t *testing.T) {
		svc := &fakeMailboxService{listErr: errors.New("list boom")}
		stubDialMail(t, svc, nil)
		root, out := newMailboxEnv(t)
		_, err := execute(t, root, out, "email", "mailbox", "list")
		require.ErrorContains(t, err, "list boom")
		assert.True(t, svc.closed, "service is closed even when the list call fails")
	})
}
