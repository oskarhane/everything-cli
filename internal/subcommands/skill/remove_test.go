package skill

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRemovePrintsRemovedLinesAndHint: after an install, remove prints one
// plain line per target and a stderr hint to reinstall.
func TestRemovePrintsRemovedLinesAndHint(t *testing.T) {
	cfg, root, out, errOut := newSkillEnv(t)
	seedAgentDir(t, cfg.Fs, "claude-code")

	_, _, err := execute(t, root, out, errOut, "skill", "install")
	require.NoError(t, err)

	stdout, stderr, err := execute(t, root, out, errOut, "skill", "remove")
	require.NoError(t, err)
	assert.Contains(t, stdout, "removed everything-cli from "+installDst(t, "claude-code")+"\n")
	assert.Contains(t, stderr, "run 'everything-cli skill install' to reinstall")

	exists, ferr := afero.Exists(cfg.Fs, installDst(t, "claude-code")+"/SKILL.md")
	require.NoError(t, ferr)
	assert.False(t, exists, "bundle must be gone after remove")
}

// TestRemoveIdempotentNoopLines: removing without an install prints a no-op
// line per target and no reinstall hint.
func TestRemoveNoOpLines(t *testing.T) {
	cfg, root, out, errOut := newSkillEnv(t)
	seedAgentDir(t, cfg.Fs, "claude-code")

	stdout, stderr, err := execute(t, root, out, errOut, "skill", "remove")
	require.NoError(t, err)

	assert.Contains(t, stdout, "no everything-cli install in "+installDst(t, "claude-code")+"\n")
	assert.NotContains(t, stdout, "removed everything-cli")
	assert.Empty(t, stderr, "no-op removal must not print the reinstall hint")
}

// TestRemoveNothingDetected: with no agents on disk, remove is a silent
// no-op success.
func TestRemoveNothingDetected(t *testing.T) {
	_, root, out, _ := newSkillEnv(t)

	stdout, _, err := execute(t, root, out, nil, "skill", "remove")
	require.NoError(t, err)
	assert.Empty(t, stdout)
}

// TestRemoveAgentFilter: --agent scopes removal and matches
// case-insensitively; other agents keep their install.
func TestRemoveAgentFilterCaseInsensitive(t *testing.T) {
	cfg, root, out, errOut := newSkillEnv(t)
	seedAgentDir(t, cfg.Fs, "claude-code")
	seedAgentDir(t, cfg.Fs, "codex")

	_, _, err := execute(t, root, out, nil, "skill", "install")
	require.NoError(t, err)

	stdout, stderr, err := execute(t, root, out, errOut, "skill", "remove", "--agent", "CODEX")
	require.NoError(t, err)
	assert.Contains(t, stdout, installDst(t, "codex"))
	assert.NotContains(t, stdout, installDst(t, "claude-code"))
	assert.Contains(t, stderr, "everything-cli skill install")

	exists, err := afero.Exists(cfg.Fs, installDst(t, "codex")+"/SKILL.md")
	require.NoError(t, err)
	assert.False(t, exists, "filtered agent's bundle must be gone")

	exists, err = afero.Exists(cfg.Fs, installDst(t, "claude-code")+"/SKILL.md")
	require.NoError(t, err)
	assert.True(t, exists, "unfiltered agent keeps its bundle")
}

// TestRemoveUnknownAgent: an unknown --agent id fails with the valid ids.
func TestRemoveUnknownAgent(t *testing.T) {
	_, root, out, _ := newSkillEnv(t)

	_, _, err := execute(t, root, out, nil, "skill", "remove", "--agent", "bogus")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "claude-code")
}

// TestRemoveFilteredUndetectedAgentIsNoOp: removing from a known agent whose
// dir is missing succeeds without a removal line.
func TestRemoveFilteredUndetectedAgent(t *testing.T) {
	_, root, out, _ := newSkillEnv(t)

	stdout, _, err := execute(t, root, out, nil, "skill", "remove", "--agent", "claude-code")
	require.NoError(t, err)
	assert.Contains(t, stdout, "no everything-cli install in")
	assert.NotContains(t, stdout, "removed everything-cli from")
}

// TestRemoveDeletesWholeBundleDir: remove takes the entire everything-cli skill
// dir (including references) with it.
func TestRemoveDeletesWholeBundle(t *testing.T) {
	cfg, root, out, _ := newSkillEnv(t)
	seedAgentDir(t, cfg.Fs, "claude-code")

	_, _, err := execute(t, root, out, nil, "skill", "install")
	require.NoError(t, err)
	_, _, err = execute(t, root, out, nil, "skill", "remove")
	require.NoError(t, err)

	exists, err := afero.DirExists(cfg.Fs, installDst(t, "claude-code"))
	require.NoError(t, err)
	assert.False(t, exists, "whole bundle dir must be removed")
}

// TestRemoveRejectsPositionalArgs: remove takes no positionals.
func TestRemoveRejectsPositionalArgs(t *testing.T) {
	cfg, root, out, _ := newSkillEnv(t)
	seedAgentDir(t, cfg.Fs, "claude-code")

	_, _, err := execute(t, root, out, nil, "skill", "remove", "extra")
	require.Error(t, err)
}
