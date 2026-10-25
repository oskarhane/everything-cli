package linear

import (
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/output"
	"github.com/oskarhane/everything-cli/internal/provider"
)

// TestMain neutralizes format auto-detection so the host's harness env and
// TTY cannot flip output expectations.
func TestMain(m *testing.M) {
	output.IsAgent = func() bool { return false }
	output.StdoutIsTerminal = func() bool { return false }
	os.Exit(m.Run())
}

func TestProviderRegistersItself(t *testing.T) {
	p, ok := provider.Get("linear")
	require.True(t, ok)
	require.Equal(t, "linear", p.ID())
}

func TestNewCmdExposesResourceTrees(t *testing.T) {
	cmd := Provider{}.NewCmd(nil)
	names := map[string]bool{}
	for _, sub := range cmd.Commands() {
		names[sub.Name()] = true
	}
	for _, want := range []string{"issue", "team", "project", "file", "account"} {
		require.True(t, names[want], "missing subtree %q", want)
	}
}

// TestIssueTreeExposesImageSubtree: the issue subtree wires the image
// commands — built through issue.Dialers' File dialer — so `linear issue
// image list|download` exist on the mounted tree.
func TestIssueTreeExposesImageSubtree(t *testing.T) {
	cmd := Provider{}.NewCmd(nil)
	var issueCmd *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Name() == "issue" {
			issueCmd = sub
		}
	}
	require.NotNil(t, issueCmd, "issue subtree missing")

	names := map[string]bool{}
	for _, sub := range issueCmd.Commands() {
		names[sub.Name()] = true
	}
	require.True(t, names["image"], "issue subtree missing image")
}
