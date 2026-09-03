package email

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
			fake := listFake(listEnvelopesFixture(), nil)
			stubDial(t, &dialMail, fake, nil)
			_, root, out := newEmailEnv(t)

			stdout, err := execute(t, root, out, "email", "message", "list", "--format", tt.format)
			require.NoError(t, err)
			tt.assert(t, stdout)
			assert.True(t, fake.closed, "the leaf must close the service it dialed")
		})
	}
}

func TestMessageListEmptyMailboxRendersEmptyArray(t *testing.T) {
	stubDial(t, &dialMail, listFake(nil, nil), nil)
	_, root, out := newEmailEnv(t)

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
			fake := listFake(nil, nil)
			stubDial(t, &dialMail, fake, nil)
			_, root, out := newEmailEnv(t)

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
		stubDial(t, &dialMail, nil, errors.New("dial boom"))
		_, root, out := newEmailEnv(t)

		_, err := execute(t, root, out, "email", "message", "list")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "dial boom")
	})

	t.Run("list failure surfaces", func(t *testing.T) {
		fake := listFake(nil, errors.New("list boom"))
		stubDial(t, &dialMail, fake, nil)
		_, root, out := newEmailEnv(t)

		_, err := execute(t, root, out, "email", "message", "list")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "list boom")
		assert.True(t, fake.closed, "a dialed service is closed even when listing fails")
	})

	t.Run("positional args are rejected", func(t *testing.T) {
		stubDial(t, &dialMail, listFake(nil, nil), nil)
		_, root, out := newEmailEnv(t)

		_, err := execute(t, root, out, "email", "message", "list", "INBOX")
		require.Error(t, err)
	})
}
