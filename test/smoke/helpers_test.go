//go:build smoke

package smoke

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/auth"
	"github.com/oskarhane/google-cli/internal/config"
	"github.com/oskarhane/google-cli/internal/output"
	"github.com/oskarhane/google-cli/internal/subcommands/account"
	"github.com/oskarhane/google-cli/internal/subcommands/calendar"
	"github.com/oskarhane/google-cli/internal/subcommands/gmail"
)

// The smoke suite is read-only: it only ever invokes the three read commands
// (account list, gmail label list, calendar list). Write endpoints — label
// and calendar create/update/delete, acl, message/draft send, anything that
// mutates the real account — are deliberately out of scope and must never be
// added here without a separate review.

func TestMain(m *testing.M) {
	// Pin the agent-harness seam so the host's harness env (CLAUDECODE, ...)
	// cannot flip format expectations. Every smoke command passes an explicit
	// --format flag, which wins over detection anyway; this is belt and
	// braces, mirroring the convention in the colocated unit tests.
	output.IsAgent = func() bool { return false }
	os.Exit(m.Run())
}

// newRootCommand mounts the real root command tree with the account, gmail,
// and calendar subtrees attached, the way main.go will once it wires them.
// cfg uses the real OS filesystem and the production config-dir resolution
// ($GOOGLE_CLI_CONFIG_DIR, else ~/.config/google-cli), which is the point of
// the smoke suite.
func newRootCommand() *cobra.Command {
	cfg := app.NewConfig()
	root := app.NewRootCommand(cfg)
	root.AddCommand(account.NewCmd(cfg))
	root.AddCommand(gmail.NewCmd(cfg))
	root.AddCommand(calendar.NewCmd(cfg))
	return root
}

// runCommand executes args against the real root command tree and returns the
// captured output. Auth failures are environment problems, so they skip
// instead of fail: a missing, expired, or unrefreshable token says nothing
// about the code under test.
func runCommand(t *testing.T, args ...string) string {
	t.Helper()

	root := newRootCommand()
	var out strings.Builder
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		if isAuthError(err) {
			t.Skipf("auth not usable in this environment (environment problem, not a test failure): %v", err)
		}
		t.Fatalf("google-cli %s: %v", strings.Join(args, " "), err)
	}
	return out.String()
}

// requireAccount skips unless the resolved config dir holds at least one
// account, and returns the account to act as: the store's default account,
// else the first listed one.
func requireAccount(t *testing.T) string {
	t.Helper()

	store, err := config.NewStore(afero.NewOsFs(), "")
	require.NoError(t, err, "resolving config dir")
	accounts, err := store.List()
	require.NoError(t, err, "listing accounts in %s", store.Dir())
	if len(accounts) == 0 {
		t.Skip("no account configured — run google-cli account add first")
	}
	def, err := store.DefaultAccount()
	require.NoError(t, err, "reading default account")
	if def != "" {
		return def
	}
	return accounts[0].Name
}

// requireCredentials skips unless OAuth app credentials resolve the way the
// gmail/calendar dialers resolve them: --credentials, then the config dir,
// then the working directory.
func requireCredentials(t *testing.T) {
	t.Helper()

	store, err := config.NewStore(afero.NewOsFs(), "")
	require.NoError(t, err, "resolving config dir")
	if _, err := auth.ResolveCredentials(afero.NewOsFs(), "", store.Dir()); err != nil {
		t.Skipf("no OAuth credentials resolved (environment problem, not a test failure): %v", err)
	}
}

// authErrorMarkers match the errors the dial path — and the lazy token
// refresh on the first API call — produces when the environment, not the
// code, is broken: missing credentials, missing account, or an expired token
// whose refresh fails.
var authErrorMarkers = []string{
	"no OAuth credentials",              // credentials resolution found nothing
	"no Google accounts configured",     // account resolution
	"no default account set",            // account resolution
	"reading credentials",               // credentials file unreadable
	"refreshing token",                  // expired token, refresh failed (dialer path)
	"cannot fetch token",                // refresh failed (API client transport path: "auth:"/"oauth2:" prefixes)
	"invalid_client",                    // refresh rejected: unknown OAuth client
	"invalid_grant",                     // revoked/expired refresh token
	"Token has been expired or revoked", // Google's refresh rejection text
	"googleapi: Error 401",              // API rejects the access token
	"Invalid Credentials",
}

// isAuthError reports whether err is an auth dial problem.
func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	for _, marker := range authErrorMarkers {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	return false
}

// decodeRows unmarshals a command's JSON output into row objects.
func decodeRows(t *testing.T, out, cmd string) []map[string]any {
	t.Helper()

	var rows []map[string]any
	err := json.Unmarshal([]byte(out), &rows)
	require.NoError(t, err, "parsing %s JSON output; output was:\n%s", cmd, out)
	return rows
}

// requireSnakeCase asserts every key in every row is lower snake_case, per
// the output casing convention (snake_case JSON keys).
func requireSnakeCase(t *testing.T, cmd string, rows []map[string]any) {
	t.Helper()

	for i, row := range rows {
		for key := range row {
			require.True(t, isSnakeCase(key), "%s row %d: key %q is not snake_case", cmd, i, key)
		}
	}
}

// isSnakeCase reports whether s is non-empty lower snake_case: lowercase
// letters, digits, and underscores only.
func isSnakeCase(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '_':
		default:
			return false
		}
	}
	return true
}
