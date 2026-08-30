// Package cmdtest holds the shared test scaffolding used by every
// subcommand package's test suite: command execution, JSON decoding, key
// assertions, and hermetic config construction. It is a plain internal
// package (not a _test.go file) so the leaf packages can import it from
// their colocated tests; importing "testing" from non-test code is the
// standard pattern for shared Go test helpers — the helpers only run under
// tests, and each takes *testing.T so failures name the right test.
package cmdtest

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/google-cli/internal/app"
)

// NewTestConfig returns a config forcing the given explicit output format
// on an in-memory FS.
func NewTestConfig(format string) *app.Config {
	return &app.Config{Format: format, Fs: afero.NewMemMapFs()}
}

// RunCmd executes a leaf cmd with its positional args and flags, returning
// everything it wrote. args must NOT include the leaf's own name: SetArgs
// feeds a single command, not a command path.
func RunCmd(t *testing.T, cmd *cobra.Command, args ...string) string {
	t.Helper()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)
	require.NoError(t, cmd.Execute())
	return buf.String()
}

// RunCmdErr executes cmd expecting failure, returning the error and output.
func RunCmdErr(t *testing.T, cmd *cobra.Command, args ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	require.Error(t, err)
	return buf.String(), err
}

// DecodeJSON unmarshals one JSON document.
func DecodeJSON(t *testing.T, s string) any {
	t.Helper()
	var v any
	require.NoError(t, json.Unmarshal([]byte(s), &v))
	return v
}

// JSONKeys returns the keys of a decoded JSON object.
func JSONKeys(t *testing.T, raw map[string]any) []string {
	t.Helper()
	keys := make([]string, 0, len(raw))
	for k := range raw {
		keys = append(keys, k)
	}
	return keys
}

// RequireSnakeCase asserts every key is lower snake_case, the output casing
// contract for JSON and TOON.
func RequireSnakeCase(t *testing.T, keys []string) {
	t.Helper()
	for _, k := range keys {
		require.Regexp(t, `^[a-z0-9_]+$`, k, "key %q must be lower snake_case", k)
	}
}
