package account

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// findCommand returns the direct subcommand named name, or nil.
func findCommand(cmd *cobra.Command, name string) *cobra.Command {
	for _, sub := range cmd.Commands() {
		if sub.Name() == name {
			return sub
		}
	}
	return nil
}

// TestParentMountsLeaves: the account parent mounts exactly the five leaves
// (cobra lists subcommands sorted), with no leaf bodies in the parent.
func TestParentMountsLeaves(t *testing.T) {
	_, root, _ := newAccountEnv(t)
	parent := findCommand(root, "account")
	require.NotNil(t, parent)

	names := make([]string, 0, len(parent.Commands()))
	for _, sub := range parent.Commands() {
		names = append(names, sub.Name())
	}
	assert.Equal(t, []string{"add", "get", "list", "remove", "use"}, names)
}

// TestLeafExamplesHaveInvocations: every leaf documents itself with a
// flush-left example block holding at least two google-cli invocations, and
// the read leaves show --format json.
func TestLeafExamplesHaveInvocations(t *testing.T) {
	_, root, _ := newAccountEnv(t)
	parent := findCommand(root, "account")
	require.NotNil(t, parent)

	for _, leaf := range []string{"list", "add", "get", "use", "remove"} {
		sub := findCommand(parent, leaf)
		require.NotNil(t, sub, "leaf %s should be mounted", leaf)
		require.NotEmpty(t, sub.Example, "%s should document examples", leaf)
		assert.True(t,
			strings.HasPrefix(sub.Example, "#") || strings.HasPrefix(sub.Example, "google-cli"),
			"%s example should be flush-left", leaf)

		invocations := 0
		for _, line := range strings.Split(sub.Example, "\n") {
			if strings.HasPrefix(line, "google-cli ") {
				invocations++
			}
		}
		assert.GreaterOrEqual(t, invocations, 2,
			"%s example should show at least two google-cli invocations", leaf)
	}

	for _, leaf := range []string{"list", "get"} {
		sub := findCommand(parent, leaf)
		require.NotNil(t, sub)
		assert.Contains(t, sub.Example, "--format json",
			"%s is a read and should show --format json", leaf)
	}
}
