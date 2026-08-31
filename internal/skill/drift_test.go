package skill_test

import (
	"io/fs"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/google-cli/internal/app"
	skillapi "github.com/oskarhane/google-cli/internal/skill"
	"github.com/oskarhane/google-cli/internal/subcommands/account"
	"github.com/oskarhane/google-cli/internal/subcommands/calendar"
	"github.com/oskarhane/google-cli/internal/subcommands/gmail"
	skillsub "github.com/oskarhane/google-cli/internal/subcommands/skill"
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
		leafPath := strings.TrimPrefix(cmd.CommandPath(), "google-cli ")
		if !strings.Contains(bundleText, leafPath) {
			t.Errorf("command %q is not documented in the skill bundle — "+
				"update internal/skill/bundle so the shipped docs match the tree", leafPath)
		}
	})
	if leafCount == 0 {
		t.Fatal("mounted tree has no runnable leaves; drift guard would pass vacuously")
	}
}

// newMountedTree mounts the complete command tree the same way main.go does.
func newMountedTree() *cobra.Command {
	cfg := &app.Config{Fs: afero.NewMemMapFs()}
	root := app.NewRootCommand(cfg)
	root.AddCommand(
		account.NewCmd(cfg),
		gmail.NewCmd(cfg),
		calendar.NewCmd(cfg),
		skillsub.NewCmd(cfg),
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

// bundleText concatenates every file in the embedded Bundle — SKILL.md plus
// all references — in the same order skill print emits them.
func bundleText(t *testing.T) string {
	t.Helper()

	skillMD, err := fs.ReadFile(skillapi.Bundle, "SKILL.md")
	require.NoError(t, err)

	var b strings.Builder
	b.Write(skillMD)

	refs, err := fs.Glob(skillapi.Bundle, "references/*.md")
	require.NoError(t, err)
	slices.Sort(refs)
	for _, name := range refs {
		data, rerr := fs.ReadFile(skillapi.Bundle, name)
		require.NoError(t, rerr)
		b.WriteString("\n===== " + name + " =====\n")
		b.Write(data)
	}
	return b.String()
}
