//go:build smoke

package smoke

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSmokeGmailLabelList runs `gmail label list` against the real account
// and asserts valid JSON with at least one label and snake_case keys.
// Read-only: label list is a read; no write endpoint is invoked.
func TestSmokeGmailLabelList(t *testing.T) {
	acct := requireAccount(t)
	requireCredentials(t)

	out := runCommand(t, "gmail", "label", "list", "--format", "json", "--account", acct)

	rows := decodeRows(t, out, "gmail label list")
	require.NotEmpty(t, rows, "expected at least one label")
	requireSnakeCase(t, "gmail label list", rows)
	require.Contains(t, rows[0], "id", "first label row lacks an id key")
	require.Contains(t, rows[0], "name", "first label row lacks a name key")
}
