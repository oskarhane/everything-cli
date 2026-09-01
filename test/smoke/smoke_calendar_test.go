//go:build smoke

package smoke

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/google-cli/internal/subcommands/cmdtest"
)

// TestSmokeCalendarList runs `calendar list` against the real account and
// asserts valid JSON rows carrying id and summary keys. Read-only: calendar
// list is a read; no write endpoint is invoked.
func TestSmokeCalendarList(t *testing.T) {
	acct := requireAccount(t)
	requireCredentials(t)

	out := runCommand(t, "google", "calendar", "list", "--format", "json", "--account", acct)

	rows := decodeRows(t, out, "calendar list")
	require.NotEmpty(t, rows, "expected at least one calendar")
	for _, row := range rows {
		cmdtest.RequireSnakeCase(t, cmdtest.JSONKeys(t, row))
	}
	require.Contains(t, rows[0], "id", "first calendar row lacks an id key")
	require.Contains(t, rows[0], "summary", "first calendar row lacks a summary key")
}
