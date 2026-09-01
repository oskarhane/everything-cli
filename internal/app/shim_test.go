package app

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRewriteLegacyArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		want     []string
		wantWarn bool
	}{
		{name: "empty", args: nil, want: nil},
		{
			name:     "bare google resource",
			args:     []string{"gmail", "list"},
			want:     []string{"google", "gmail", "list"},
			wantWarn: true,
		},
		{
			name:     "youtube resource",
			args:     []string{"youtube", "metadata", "abc"},
			want:     []string{"google", "youtube", "metadata", "abc"},
			wantWarn: true,
		},
		{
			name:     "moved account verb",
			args:     []string{"account", "add", "work"},
			want:     []string{"google", "account", "add", "work"},
			wantWarn: true,
		},
		{
			name:     "account remove with flags",
			args:     []string{"account", "remove", "old", "--force"},
			want:     []string{"google", "account", "remove", "old", "--force"},
			wantWarn: true,
		},
		{
			name: "account list stays top-level",
			args: []string{"account", "list"},
			want: []string{"account", "list"},
		},
		{
			name: "bare account stays",
			args: []string{"account"},
			want: []string{"account"},
		},
		{
			name: "already provider-first",
			args: []string{"google", "gmail", "list"},
			want: []string{"google", "gmail", "list"},
		},
		{
			name: "cli-own command untouched",
			args: []string{"skill", "print"},
			want: []string{"skill", "print"},
		},
		{
			name: "flags first untouched",
			args: []string{"--format", "json", "gmail", "list"},
			want: []string{"--format", "json", "gmail", "list"},
		},
		{
			name: "help untouched",
			args: []string{"--help"},
			want: []string{"--help"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var errBuf bytes.Buffer
			got := RewriteLegacyArgs("google-cli", tt.args, &errBuf)
			assert.Equal(t, tt.want, got)
			if tt.wantWarn {
				assert.Contains(t, errBuf.String(), "deprecated")
				assert.Contains(t, errBuf.String(), "google-cli google "+joinArgs(tt.args))
			} else {
				assert.Empty(t, errBuf.String(), "no warning without a rewrite")
			}
		})
	}
}

// TestRewriteLegacyArgsDoesNotMutateInput: the rewritten slice must not
// share backing storage with the caller's argv.
func TestRewriteLegacyArgsDoesNotMutateInput(t *testing.T) {
	args := []string{"gmail", "list"}
	var errBuf bytes.Buffer
	got := RewriteLegacyArgs("google-cli", args, &errBuf)
	got[1] = "mutated"
	assert.Equal(t, "gmail", args[0])
	assert.Equal(t, "list", args[1])
}
