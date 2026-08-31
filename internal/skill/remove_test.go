package skill

import (
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRemoveIdempotentMissingDir: removing from a detected agent with no
// installed bundle succeeds (nothing to do).
func TestRemoveIdempotentMissingDir(t *testing.T) {
	home := t.TempDir()
	fsys := newTestFS(t, home)
	seedAgents(t, fsys, "claude-code")

	removed, err := Remove(fsys, "")
	require.NoError(t, err)
	require.Len(t, removed, 1)
	assert.Equal(t, "claude-code", removed[0].Agent)
	assert.False(t, removed[0].Removed, "no install dir means a no-op removal")
	assert.Equal(t, destFor(*FindAgent("claude-code"), home), removed[0].Path)
}

// TestRemoveAgentScoping: with a filter, only the named agent's install
// is removed.
func TestRemoveAgentScoping(t *testing.T) {
	home := t.TempDir()
	fsys := newTestFS(t, home)
	seedAgents(t, fsys, "claude-code", "codex")

	for _, name := range []string{"claude-code", "codex"} {
		dest := destFor(*FindAgent(name), home)
		require.NoError(t, fsys.MkdirAll(dest, 0o755))
	}

	removed, err := Remove(fsys, "codex")
	require.NoError(t, err)
	require.Len(t, removed, 1)
	assert.Equal(t, "codex", removed[0].Agent)
	assert.True(t, removed[0].Removed, "existing install dir means a real removal")
	assert.Equal(t, destFor(*FindAgent("codex"), home), removed[0].Path)

	exists, err := afero.DirExists(fsys, destFor(*FindAgent("codex"), home))
	require.NoError(t, err)
	assert.False(t, exists, "filtered agent's install dir must be gone")

	exists, err = afero.DirExists(fsys, destFor(*FindAgent("claude-code"), home))
	require.NoError(t, err)
	assert.True(t, exists, "unfiltered agent's install dir must remain")
}

// TestRemoveUnknownAgent: an unknown filter is an error.
func TestRemoveUnknownAgent(t *testing.T) {
	home := t.TempDir()
	fsys := newTestFS(t, home)

	_, err := Remove(fsys, "claude-desktop")
	require.ErrorIs(t, err, ErrUnknownAgent)
}

// TestRemoveAllDetected: no filter removes every detected agent's install
// dir; zero detected agents is a no-op, not an error.
func TestRemoveAllDetected(t *testing.T) {
	t.Run("all detected", func(t *testing.T) {
		home := t.TempDir()
		fsys := newTestFS(t, home)
		seedAgents(t, fsys, "claude-code", "junie")

		for _, name := range []string{"claude-code", "junie"} {
			dest := destFor(*FindAgent(name), home)
			require.NoError(t, fsys.MkdirAll(dest, 0o755))
		}

		removed, err := Remove(fsys, "")
		require.NoError(t, err)
		assert.Len(t, removed, 2)

		for _, name := range []string{"claude-code", "junie"} {
			exists, ferr := afero.DirExists(fsys, destFor(*FindAgent(name), home))
			require.NoError(t, ferr)
			assert.False(t, exists, name+" install dir must be gone")
		}
	})

	t.Run("zero detected is a no-op", func(t *testing.T) {
		home := t.TempDir()
		fsys := newTestFS(t, home)

		removed, err := Remove(fsys, "")
		require.NoError(t, err)
		assert.Empty(t, removed)
	})
}

// TestRemoveCaseInsensitiveFilter: the filter matches case-insensitively.
func TestRemoveCaseInsensitiveFilter(t *testing.T) {
	home := t.TempDir()
	fsys := newTestFS(t, home)
	seedAgents(t, fsys, "claude-code")

	removed, err := Remove(fsys, "Claude-Code")
	require.NoError(t, err)
	require.Len(t, removed, 1)
	assert.Equal(t, "claude-code", removed[0].Agent)
}

// TestRemoveDeletesWholeSkillDir: removal takes out the bundle dir, not
// just SKILL.md.
func TestRemoveDeletesWholeSkillDir(t *testing.T) {
	home := t.TempDir()
	fsys := newTestFS(t, home)
	seedAgents(t, fsys, "claude-code")

	dest := filepath.Join(destFor(*FindAgent("claude-code"), home), "references")
	require.NoError(t, fsys.MkdirAll(dest, 0o755))

	_, err := Remove(fsys, "claude-code")
	require.NoError(t, err)

	exists, ferr := afero.DirExists(fsys, filepath.Dir(dest))
	require.NoError(t, ferr)
	assert.False(t, exists, "whole <skills>/google-cli dir must be gone")
}
