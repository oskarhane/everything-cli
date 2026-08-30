// Package gates holds repo-wide WHOLE-TREE gate tests. They mount the full
// google-cli command tree exactly the way main.go does (main itself cannot be
// imported) and walk it with cobra, so every current and future subtree is
// covered automatically without executing anything — hermetic by design.
package gates

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/subcommands/account"
	"github.com/oskarhane/google-cli/internal/subcommands/calendar"
	"github.com/oskarhane/google-cli/internal/subcommands/gmail"
)

// newWholeTree mounts the complete command tree the same way main.go does,
// with an in-memory FS so nothing touches real credential paths.
func newWholeTree() *cobra.Command {
	cfg := &app.Config{Fs: afero.NewMemMapFs()}
	root := app.NewRootCommand(cfg)
	root.AddCommand(
		account.NewCmd(cfg),
		gmail.NewCmd(cfg),
		calendar.NewCmd(cfg),
	)
	return root
}

// walkTree visits cmd and every descendant, depth-first.
func walkTree(cmd *cobra.Command, visit func(*cobra.Command)) {
	visit(cmd)
	for _, sub := range cmd.Commands() {
		walkTree(sub, visit)
	}
}

// isRunnableLeaf reports whether cmd is a leaf with an action to run.
func isRunnableLeaf(cmd *cobra.Command) bool {
	return len(cmd.Commands()) == 0 && (cmd.Run != nil || cmd.RunE != nil)
}

// useFirstWord returns the command name in Use ("get <id>" -> "get").
func useFirstWord(use string) string {
	first, _, _ := strings.Cut(use, " ")
	return first
}

// countInvocations counts flush-left example lines invoking google-cli.
func countInvocations(example string) int {
	n := 0
	for _, line := range strings.Split(example, "\n") {
		if strings.HasPrefix(line, "google-cli ") {
			n++
		}
	}
	return n
}

// subcommandsDir resolves the repo root from the test's working directory
// (the package dir) so the source scanner can find internal/subcommands.
func subcommandsDir(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, "internal", "subcommands")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not locate go.mod upward from test package dir")
		}
		dir = parent
	}
}
