//go:build smoke

package smoke

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

// TestSmokeAccountList runs `account list` against the real config dir and
// asserts exit 0 plus non-empty, parseable output. Read-only: account list
// only reads the store; no write endpoint is invoked.
func TestSmokeAccountList(t *testing.T) {
	requireAccount(t)

	out := runCommand(t, "account", "list", "--format", "json")

	require.NotEmpty(t, strings.TrimSpace(out), "account list printed nothing")
	rows := decodeRows(t, out, "account list")
	require.NotEmpty(t, rows, "expected at least one account row")
	for _, row := range rows {
		cmdtest.RequireSnakeCase(t, cmdtest.JSONKeys(t, row))
	}
}
