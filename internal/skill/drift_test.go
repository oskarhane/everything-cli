package skill_test

import (
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/google-cli/internal/app"
	googleprovider "github.com/oskarhane/google-cli/internal/providers/google"
	skillapi "github.com/oskarhane/google-cli/internal/skill"
	"github.com/oskarhane/google-cli/internal/subcommands/account"
	skillsub "github.com/oskarhane/google-cli/internal/subcommands/skill"
	"github.com/oskarhane/google-cli/internal/subcommands/update"
)

// TestTreeDrift is the drift guard: every runnable leaf of the mounted
// google-cli command tree (built exactly as main.go builds it) must be
// documented in the embedded skill bundle, so the shipped SKILL.md can
// never silently fall behind the actual command surface.
//
// It is fully hermetic: the bundle is an embedded FS, the tree is mounted
// on an in-memory FS, and no environment is read.
func TestTreeDrift(t *testing.T) {
	bundleText := bundleText(t)

	root := newMountedTree()
	leafCount := 0
	walkTree(root, func(cmd *cobra.Command) {
		if len(cmd.Commands()) != 0 || (cmd.Run == nil && cmd.RunE == nil) {
			return
		}
		leafCount++
		// Multi-word leaf paths ("gmail message untrash") are specific enough
		// to match as written; single-word leaves ("update") match
		// incidentally in prose, so they must appear with the binary prefix
		// ("google-cli update") for the guard to bite.
		leafPath := strings.TrimPrefix(cmd.CommandPath(), "google-cli ")
		// The bundle still documents the pre-provider command surface; the
		// provider segment ("google gmail list" -> "gmail list") is stripped
		// until the skill-sync node rewrites the bundle provider-first.
		leafPath = strings.TrimPrefix(leafPath, "google ")
		if !strings.Contains(leafPath, " ") {
			leafPath = "google-cli " + leafPath
		}
		if !strings.Contains(bundleText, leafPath) {
			t.Errorf("command %q is not documented in the skill bundle — "+
				"update internal/skill/bundle so the shipped docs match the tree", leafPath)
		}
	})
	if leafCount == 0 {
		t.Fatal("mounted tree has no runnable leaves; drift guard would pass vacuously")
	}
}

// newMountedTree mounts the complete command tree the same way main.go does:
// provider trees under their provider command, CLI-own commands top-level.
func newMountedTree() *cobra.Command {
	cfg := &app.Config{Fs: afero.NewMemMapFs()}
	root := app.NewRootCommand(cfg)
	root.AddCommand(
		googleprovider.Provider{}.NewCmd(cfg),
		account.NewCmd(cfg),
		skillsub.NewCmd(cfg),
		update.NewCmd(cfg),
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

// bundleText renders the embedded bundle through the single owner of the
// print contract (skillapi.Render) — the same bytes skill print emits.
func bundleText(t *testing.T) string {
	t.Helper()

	var b strings.Builder
	require.NoError(t, skillapi.Render(&b))
	return b.String()
}
