package skill

import (
	"testing"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInstallWritesBundleAndPrintsLines: a plain install copies the bundle
// into every detected agent's skills dir and prints one plain line per
// target — no structured output, no --format involvement.
func TestInstallWritesBundleAndPrintsLines(t *testing.T) {
	cfg, root, out, _ := newSkillEnv(t)
	seedAgentDir(t, cfg.Fs, "claude-code")
	seedAgentDir(t, cfg.Fs, "codex")

	stdout, _, err := execute(t, root, out, nil, "skill", "install")
	require.NoError(t, err)

	assert.Contains(t, stdout, "installed google-cli -> "+installDst(t, "claude-code")+"\n")
	assert.Contains(t, stdout, "installed google-cli -> "+installDst(t, "codex")+"\n")

	exists, err := afero.Exists(cfg.Fs, installDst(t, "claude-code")+"/SKILL.md")
	require.NoError(t, err)
	assert.True(t, exists, "bundle SKILL.md must land on the in-memory FS")

	exists, err = afero.Exists(cfg.Fs, installDst(t, "claude-code")+"/references/gmail.md")
	require.NoError(t, err)
	assert.True(t, exists, "references must be installed too")
}

// TestInstallCaseInsensitiveAgentFilter: --agent matches ids
// case-insensitively and installs only that agent.
func TestInstallCaseInsensitiveAgentFilter(t *testing.T) {
	cfg, root, out, _ := newSkillEnv(t)
	seedAgentDir(t, cfg.Fs, "claude-code")
	seedAgentDir(t, cfg.Fs, "codex")

	stdout, _, err := execute(t, root, out, nil, "skill", "install", "--agent", "CLAUDE-CODE")
	require.NoError(t, err)
	assert.Contains(t, stdout, installDst(t, "claude-code"))
	assert.NotContains(t, stdout, installDst(t, "codex"))
}

// TestInstallVersionIsStamped: the installed SKILL.md carries the built
// binary's version, not the upstream placeholder.
func TestInstallVersionIsStamped(t *testing.T) {
	cfg, root, out, _ := newSkillEnv(t)
	seedAgentDir(t, cfg.Fs, "claude-code")

	_, _, err := execute(t, root, out, nil, "skill", "install")
	require.NoError(t, err)

	data, err := afero.ReadFile(cfg.Fs, installDst(t, "claude-code")+"/SKILL.md")
	require.NoError(t, err)
	assert.Contains(t, string(data), "version: "+app.Version)
}

// TestInstallUnknownAgentNamesValidOnes: an unknown --agent id fails with a
// usage-style error listing every supported agent id.
func TestInstallUnknownAgent(t *testing.T) {
	_, root, out, errOut := newSkillEnv(t)

	_, _, err := execute(t, root, out, errOut, "skill", "install", "--agent", "bogus")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown agent")
	assert.Contains(t, err.Error(), "claude-code")
	assert.Contains(t, err.Error(), "junie")
	assert.NotContains(t, err.Error(), "Claude Code", "ids are lowercase catalog names")
}

// TestInstallNoAgentsDetected: with no agent dirs on disk and no --agent,
// install fails with a hint pointing at creating an agent dir or --agent.
func TestInstallNoAgentsDetected(t *testing.T) {
	_, root, out, _ := newSkillEnv(t)

	_, _, err := execute(t, root, out, nil, "skill", "install")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--agent")
	assert.Contains(t, err.Error(), ".claude")
}

// TestInstallKnownButUndetectedAgent: a known agent whose DetectDir is
// missing is an error, not a silent no-op.
func TestInstallKnownButUndetectedAgent(t *testing.T) {
	_, root, out, _ := newSkillEnv(t)

	_, _, err := execute(t, root, out, nil, "skill", "install", "--agent", "claude-code")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "claude-code")
}

// TestInstallRejectsPositionalArgs: install takes no positionals.
func TestInstallRejectsPositionals(t *testing.T) {
	cfg, root, out, _ := newSkillEnv(t)
	seedAgentDir(t, cfg.Fs, "claude-code")

	_, _, err := execute(t, root, out, nil, "skill", "install", "extra")
	require.Error(t, err)
}

// TestInstallRefreshesPriorInstall: re-installing over an existing install
// leaves exactly one bundle with fresh content (clean-slate).
func TestInstallRefreshesPriorInstall(t *testing.T) {
	cfg, root, out, _ := newSkillEnv(t)
	seedAgentDir(t, cfg.Fs, "claude-code")

	_, _, err := execute(t, root, out, nil, "skill", "install")
	require.NoError(t, err)

	stale := installDst(t, "claude-code") + "/references/stale.md"
	require.NoError(t, afero.WriteFile(cfg.Fs, stale, []byte("stale"), 0o600))

	_, _, err = execute(t, root, out, nil, "skill", "install")
	require.NoError(t, err)

	exists, err := afero.Exists(cfg.Fs, stale)
	require.NoError(t, err)
	assert.False(t, exists, "re-install must be clean-slate")
}
