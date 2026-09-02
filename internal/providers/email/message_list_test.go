package email

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/app"
)

// fakeListService is a hermetic EnvelopeLister (plus the no-op rest of
// MailService) injected through the dialMail seam: no test ever touches
// the network. It records the arguments the leaf passed and whether the
// leaf closed the connection.
type fakeListService struct {
	envelopes  []Envelope
	listErr    error
	gotMailbox string
	gotLimit   int
	closed     bool
}

func (f *fakeListService) ListEnvelopes(_ context.Context, mailbox string, limit int) ([]Envelope, error) {
	f.gotMailbox, f.gotLimit = mailbox, limit
	return f.envelopes, f.listErr
}

func (f *fakeListService) ListMailboxes(context.Context) ([]string, error) {
	return nil, errors.New("fakeListService: not implemented")
}

func (f *fakeListService) GetMessage(context.Context, string, uint32) (*Message, error) {
	return nil, errors.New("fakeListService: not implemented")
}

func (f *fakeListService) SendMessage(context.Context, SendInput) error {
	return errors.New("fakeListService: not implemented")
}

func (f *fakeListService) Close() error {
	f.closed = true
	return nil
}

// stubListDial swaps the dialMail seam for the test's lifetime so the leaf
// dials the fake instead of IMAP.
func stubListDial(t *testing.T, svc MailService, err error) {
	t.Helper()
	saved := dialMail
	dialMail = func(context.Context, *app.Config) (MailService, error) { return svc, err }
	t.Cleanup(func() { dialMail = saved })
}

// newMessageListEnv mounts the message subtree on a fresh root directly
// rather than via Provider.NewCmd: the provider wiring is another node's
// file, and this test must be hermetic to this node's files. The dial seam
// is stubbed, so no account or config dir is ever read.
func newMessageListEnv(t *testing.T) (*cobra.Command, *bytes.Buffer) {
	t.Helper()
	cfg := &app.Config{Fs: afero.NewMemMapFs()}
	root := app.NewRootCommand(cfg)
	emailCmd := &cobra.Command{Use: "email"}
	emailCmd.AddCommand(newMessageCmd(cfg))
	root.AddCommand(emailCmd)
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(io.Discard)
	return root, out
}

// listEnvelopesFixture is two envelopes, newest first, with distinct
// values per field so a swapped or dropped field fails an assertion.
func listEnvelopesFixture() []Envelope {
	return []Envelope{
		{
			UID:     42,
			Date:    time.Date(2026, 8, 30, 14, 15, 0, 0, time.UTC),
			From:    "Alice <alice@example.com>",
			Subject: "Second, newer",
			Flags:   []string{`\Seen`},
		},
		{
			UID:     41,
			Date:    time.Date(2026, 8, 29, 9, 0, 0, 0, time.UTC),
			From:    "bob@example.com",
			Subject: "First, older",
			Flags:   []string{`\Seen`, `\Flagged`},
		},
	}
}

func TestMessageListFormats(t *testing.T) {
	tests := []struct {
		name   string
		format string
		assert func(t *testing.T, stdout string)
	}{
		{
			name:   "json renders snake_case fields with flags as an array",
			format: "json",
			assert: func(t *testing.T, stdout string) {
				var rows []map[string]any
				require.NoError(t, json.Unmarshal([]byte(stdout), &rows))
				require.Len(t, rows, 2)
				assert.Equal(t, float64(42), rows[0]["uid"])
				assert.Equal(t, "2026-08-30T14:15:00Z", rows[0]["date"])
				assert.Equal(t, "Alice <alice@example.com>", rows[0]["from"])
				assert.Equal(t, "Second, newer", rows[0]["subject"])
				assert.Equal(t, []any{`\Seen`}, rows[0]["flags"])
			},
		},
		{
			name:   "table renders upper-case headers and one line per envelope",
			format: "table",
			assert: func(t *testing.T, stdout string) {
				for _, header := range []string{"UID", "DATE", "FROM", "SUBJECT", "FLAGS"} {
					assert.Contains(t, stdout, header)
				}
				assert.Contains(t, stdout, "Second, newer")
				// Table cells join flags so a row stays one line.
				assert.Contains(t, stdout, `\Seen,\Flagged`)
			},
		},
		{
			name:   "toon renders the same snake_case keys",
			format: "toon",
			assert: func(t *testing.T, stdout string) {
				assert.Contains(t, stdout, "uid")
				assert.Contains(t, stdout, "subject")
				assert.Contains(t, stdout, "Second, newer")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeListService{envelopes: listEnvelopesFixture()}
			stubListDial(t, fake, nil)
			root, out := newMessageListEnv(t)

			stdout, err := execute(t, root, out, "email", "message", "list", "--format", tt.format)
			require.NoError(t, err)
			tt.assert(t, stdout)
			assert.True(t, fake.closed, "the leaf must close the service it dialed")
		})
	}
}

func TestMessageListEmptyMailboxRendersEmptyArray(t *testing.T) {
	stubListDial(t, &fakeListService{}, nil)
	root, out := newMessageListEnv(t)

	stdout, err := execute(t, root, out, "email", "message", "list", "--format", "json")
	require.NoError(t, err)
	assert.JSONEq(t, `[]`, stdout)
}

func TestMessageListFlagDefaultsAndPassThrough(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantMailbox string
		wantLimit   int
	}{
		{name: "defaults to INBOX and 25", args: nil, wantMailbox: "INBOX", wantLimit: 25},
		{
			name:        "passes --mailbox and --limit through",
			args:        []string{"--mailbox", "Archive", "--limit", "5"},
			wantMailbox: "Archive",
			wantLimit:   5,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeListService{}
			stubListDial(t, fake, nil)
			root, out := newMessageListEnv(t)

			args := append([]string{"email", "message", "list"}, tt.args...)
			_, err := execute(t, root, out, args...)
			require.NoError(t, err)
			assert.Equal(t, tt.wantMailbox, fake.gotMailbox)
			assert.Equal(t, tt.wantLimit, fake.gotLimit)
		})
	}
}

func TestMessageListErrors(t *testing.T) {
	t.Run("dial failure surfaces and nothing is closed", func(t *testing.T) {
		stubListDial(t, nil, errors.New("dial boom"))
		root, out := newMessageListEnv(t)

		_, err := execute(t, root, out, "email", "message", "list")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "dial boom")
	})

	t.Run("list failure surfaces", func(t *testing.T) {
		fake := &fakeListService{listErr: errors.New("list boom")}
		stubListDial(t, fake, nil)
		root, out := newMessageListEnv(t)

		_, err := execute(t, root, out, "email", "message", "list")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "list boom")
		assert.True(t, fake.closed, "a dialed service is closed even when listing fails")
	})

	t.Run("positional args are rejected", func(t *testing.T) {
		stubListDial(t, &fakeListService{}, nil)
		root, out := newMessageListEnv(t)

		_, err := execute(t, root, out, "email", "message", "list", "INBOX")
		require.Error(t, err)
	})
}
