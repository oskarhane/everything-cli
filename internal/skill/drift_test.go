package skill_test

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/cmdtree"
	"github.com/oskarhane/everything-cli/internal/provider"
	skillapi "github.com/oskarhane/everything-cli/internal/skill"

	// Provider side-effect imports mirror main.go exactly — keep this
	// list in sync with main.go's import block so the mounted tree and
	// the registry match the shipped binary.
	_ "github.com/oskarhane/everything-cli/internal/providers/email"
	_ "github.com/oskarhane/everything-cli/internal/providers/google"
	_ "github.com/oskarhane/everything-cli/internal/providers/granola"
	_ "github.com/oskarhane/everything-cli/internal/providers/linear"
)

// TestTreeDrift is the drift guard: every runnable leaf of the mounted
// everything-cli command tree (built through cmdtree.New — the same
// registry-driven assembly main.go consumes) must be documented in the
// embedded skill bundle, so the shipped SKILL.md can never silently fall
// behind the actual command surface.
//
// It is fully hermetic: the bundle is an embedded FS, the tree is mounted
// on an in-memory FS, and no environment is read.
func TestTreeDrift(t *testing.T) {
	bundleText := bundleText(t)

	root := newMountedTree()
	leafCount := 0
	cmdtree.WalkTree(root, func(cmd *cobra.Command) {
		if len(cmd.Commands()) != 0 || (cmd.Run == nil && cmd.RunE == nil) {
			return
		}
		leafCount++
		// The skill is a thin router: provider leaves are documented in
		// references/<provider>.md by their provider-first paths, which
		// contain the provider-stripped resource path as a substring —
		// strip the provider segment ("google gmail list" -> "gmail
		// list", "linear issue list" -> "issue list") before matching.
		// Multi-word leaf paths are specific enough to match as written;
		// single-word leaves ("update") match incidentally in prose, so
		// they must appear with the binary prefix ("everything-cli
		// update") for the guard to bite.
		leafPath := strings.TrimPrefix(cmd.CommandPath(), "everything-cli ")
		if first, rest, ok := strings.Cut(leafPath, " "); ok {
			if _, registered := provider.Get(first); registered {
				leafPath = rest
			}
		}
		if !strings.Contains(leafPath, " ") {
			leafPath = "everything-cli " + leafPath
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

// TestProviderDocsDrift is the provider-coverage guard: every provider
// registered in the registry (the same set main.go mounts) must be
// indexed in the thin-router SKILL.md — a link to its reference file in
// the provider index — and that references/<id>.md file must exist in
// the embedded bundle. Adding a provider to main.go without either
// piece fails this test.
func TestProviderDocsDrift(t *testing.T) {
	data, err := fs.ReadFile(skillapi.Bundle, "SKILL.md")
	require.NoError(t, err)
	skillMd := string(data)

	providers := provider.List()
	require.NotEmpty(t, providers,
		"no providers registered; provider-docs guard would pass vacuously")
	for _, p := range providers {
		ref := "references/" + p.ID() + ".md"
		// Pin the markdown link target, not the bare substring — a
		// mangled path like "references/linear.md.disabled" still
		// contains "references/linear.md".
		assert.Contains(t, skillMd, "]("+ref+")",
			"SKILL.md provider index must link %s for registered provider %q", ref, p.ID())
		_, statErr := fs.Stat(skillapi.Bundle, ref)
		assert.NoError(t, statErr,
			"bundle must ship %s for registered provider %q", ref, p.ID())
	}
}

// newMountedTree mounts the complete command tree through the shared
// registry-driven assembly (cmdtree.New — the one main.go consumes), on an
// in-memory FS so nothing touches real credential paths.
func newMountedTree() *cobra.Command {
	return cmdtree.New(&app.Config{Fs: afero.NewMemMapFs()})
}

// bundleText renders the embedded bundle through the single owner of the
// print contract (skillapi.Render) — the same bytes skill print emits.
func bundleText(t *testing.T) string {
	t.Helper()

	var b strings.Builder
	require.NoError(t, skillapi.Render(&b))
	return b.String()
}
