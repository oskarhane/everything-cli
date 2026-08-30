package event

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/google-cli/internal/subcommands/cmdtest"
)

func TestNewCmdRegistersLeaves(t *testing.T) {
	cmd := NewCmd(cmdtest.NewTestConfig("json"), fakeNewSvc(&fakeEventService{}))

	var names []string
	for _, leaf := range cmd.Commands() {
		names = append(names, leaf.Name())
	}
	require.Equal(t, []string{
		"accept", "create", "decline", "delete", "get",
		"instances", "list", "move", "tentative", "update",
	}, names)
	require.Equal(t, "event", cmd.Name())
}

// TestLeafExamples enforces the example contract on every leaf: flush-left
// comment-led examples with at least two google-cli invocations, a recurring
// example, and --format json on the read leaves.
func TestLeafExamples(t *testing.T) {
	cmd := NewCmd(cmdtest.NewTestConfig("json"), fakeNewSvc(&fakeEventService{}))
	leaves := cmd.Commands()
	require.NotEmpty(t, leaves)

	for _, leaf := range leaves {
		t.Run(leaf.Name(), func(t *testing.T) {
			example := leaf.Example
			require.NotEmpty(t, example, "every leaf needs an Example")
			require.True(t, strings.HasPrefix(example, "# "), "Example must be flush-left, starting with a # comment")

			require.GreaterOrEqual(t,
				strings.Count(example, "google-cli calendar event "+leaf.Name()),
				2, "Example needs at least two google-cli invocations")
			require.Contains(t, example, "# ", "Example needs # comments")

			// At least one recurring example per leaf.
			recurring := false
			for _, marker := range []string{"kq3abc123", "recurring", "series", "occurrence"} {
				if strings.Contains(example, marker) {
					recurring = true
					break
				}
			}
			require.True(t, recurring, "Example needs a recurring example")

			// Read leaves must show machine-readable output.
			if isReadLeaf(leaf.Name()) {
				require.Contains(t, example, "--format json")
			}
		})
	}
}

// isReadLeaf reports whether a leaf only reads, where --format json matters.
func isReadLeaf(name string) bool {
	switch name {
	case "list", "get", "instances":
		return true
	default:
		return false
	}
}

// TestLeafArgsAndFlags spot-checks the surface each leaf must expose.
func TestLeafArgsAndFlags(t *testing.T) {
	cmd := NewCmd(cmdtest.NewTestConfig("json"), fakeNewSvc(&fakeEventService{}))
	leaf := func(name string) *cobra.Command {
		for _, c := range cmd.Commands() {
			if c.Name() == name {
				return c
			}
		}
		t.Fatalf("leaf %q not registered", name)
		return nil
	}

	require.NotNil(t, leaf("create").Flags().Lookup("all-day"))
	for _, name := range []string{"update", "delete"} {
		require.NotNil(t, leaf(name).Flags().Lookup("this-only"), "%s needs --this-only", name)
	}
	for _, name := range []string{"accept", "decline", "tentative"} {
		require.NotNil(t, leaf(name).Flags().Lookup("all"), "%s needs --all", name)
	}
	require.NotNil(t, leaf("move").Flags().Lookup("to-calendar"))
	require.NotNil(t, leaf("delete").Flags().Lookup("force"))
	require.NotNil(t, leaf("list").Flags().Lookup("recurring"))
	require.NotNil(t, leaf("create").Flags().Lookup("recurrence"))
}
