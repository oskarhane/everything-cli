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
			name:     "flag-first rewrite with warning",
			args:     []string{"--format", "json", "gmail", "list"},
			want:     []string{"--format", "json", "google", "gmail", "list"},
			wantWarn: true,
		},
		{
			name:     "equals-form flag-first rewrite",
			args:     []string{"--format=json", "gmail", "list"},
			want:     []string{"--format=json", "google", "gmail", "list"},
			wantWarn: true,
		},
		{
			name:     "account flag value skipped",
			args:     []string{"--account", "work", "--debug", "drive", "list"},
			want:     []string{"--account", "work", "--debug", "google", "drive", "list"},
			wantWarn: true,
		},
		{
			name:     "credentials flag-first account verb",
			args:     []string{"--credentials=creds.json", "account", "add", "work"},
			want:     []string{"--credentials=creds.json", "google", "account", "add", "work"},
			wantWarn: true,
		},
		{
			name: "flag-first account list stays top-level",
			args: []string{"--format", "json", "account", "list"},
			want: []string{"--format", "json", "account", "list"},
		},
		{
			name: "unknown flag passes through untouched",
			args: []string{"--bogus", "gmail", "list"},
			want: []string{"--bogus", "gmail", "list"},
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
			got := RewriteLegacyArgs("everything-cli", tt.args, &errBuf)
			assert.Equal(t, tt.want, got)
			if tt.wantWarn {
				assert.Contains(t, errBuf.String(), "deprecated")
				assert.Contains(t, errBuf.String(), "everything-cli "+joinArgs(tt.args))
				assert.Contains(t, errBuf.String(), "everything-cli "+joinArgs(tt.want))
			} else {
				assert.Empty(t, errBuf.String(), "no warning without a rewrite")
			}
		})
	}
}

// TestRewriteLegacyArgsWarningStripsControlBytes: argv is echoed into the
// deprecation warning, so a crafted argument carrying an ANSI escape must
// not reach the terminal raw.
func TestRewriteLegacyArgsWarningStripsControlBytes(t *testing.T) {
	args := []string{"gmail", "list", "\x1b[31mred"}
	var errBuf bytes.Buffer
	got := RewriteLegacyArgs("everything-cli", args, &errBuf)
	assert.Equal(t, []string{"google", "gmail", "list", "\x1b[31mred"}, got,
		"the rewrite itself passes argv through untouched")
	warn := errBuf.String()
	assert.Contains(t, warn, "deprecated")
	assert.NotContains(t, warn, "\x1b", "ESC must be stripped from the echoed argv")
	assert.Contains(t, warn, "?[31mred")
}

// TestRewriteLegacyArgsDoesNotMutateInput: the rewritten slice must not
// share backing storage with the caller's argv.
func TestRewriteLegacyArgsDoesNotMutateInput(t *testing.T) {
	args := []string{"gmail", "list"}
	var errBuf bytes.Buffer
	got := RewriteLegacyArgs("everything-cli", args, &errBuf)
	got[1] = "mutated"
	assert.Equal(t, "gmail", args[0])
	assert.Equal(t, "list", args[1])
}
