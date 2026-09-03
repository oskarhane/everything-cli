package email

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
			svc := mailboxFake(tt.names, nil)
			stubDial(t, &dialMail, svc, nil)
			_, root, out := newEmailEnv(t)

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
		stubDial(t, &dialMail, nil, errors.New("dial boom"))
		_, root, out := newEmailEnv(t)
		_, err := execute(t, root, out, "email", "mailbox", "list")
		require.ErrorContains(t, err, "dial boom")
	})

	t.Run("list failure propagates and still closes", func(t *testing.T) {
		svc := mailboxFake(nil, errors.New("list boom"))
		stubDial(t, &dialMail, svc, nil)
		_, root, out := newEmailEnv(t)
		_, err := execute(t, root, out, "email", "mailbox", "list")
		require.ErrorContains(t, err, "list boom")
		assert.True(t, svc.closed, "service is closed even when the list call fails")
	})
}
