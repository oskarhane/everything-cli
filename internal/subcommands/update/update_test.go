package update

import (
	"encoding/json"
	"testing"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
	updateapi "github.com/oskarhane/everything-cli/internal/update"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setVersion pins app.Version for the duration of one test.
func setVersion(t *testing.T, v string) {
	t.Helper()
	prev := app.Version
	app.Version = v
	t.Cleanup(func() { app.Version = prev })
}

// TestUpdateCheck_PrintsVersionsAndChangesNothing: --check renders the
// version comparison and never downloads or runs an update.
func TestUpdateCheck_PrintsVersionsAndChangesNothing(t *testing.T) {
	_, root, out, _ := newUpdateEnv(t)
	setVersion(t, "v1.0.0")
	fc := &fakeClient{rel: relFixture()}
	stubClient(t, fc)
	calls := stubRun(t)

	stdout, err := execute(t, root, out, "update", "--check", "--format", "json")
	require.NoError(t, err)

	assert.Contains(t, stdout, `"current_version": "v1.0.0"`)
	assert.Contains(t, stdout, `"latest_version": "v1.2.3"`)
	assert.Contains(t, stdout, `"update_available": true`)
	assert.Empty(t, fc.downloads, "--check must not download anything")
	assert.Empty(t, *calls, "--check must not call update.Run")
}

// TestUpdateCheck_JSONKeysAreSnakeCase pins the output casing contract.
func TestUpdateCheck_JSONKeysAreSnakeCase(t *testing.T) {
	_, root, out, _ := newUpdateEnv(t)
	setVersion(t, "v1.0.0")
	stubClient(t, &fakeClient{rel: relFixture()})

	stdout, err := execute(t, root, out, "update", "--check", "--format", "json")
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &decoded))
	cmdtest.RequireSnakeCase(t, cmdtest.JSONKeys(t, decoded))
	assert.ElementsMatch(t,
		[]string{"current_version", "latest_version", "update_available"},
		cmdtest.JSONKeys(t, decoded))
}

// TestUpdateYes_AutoInstalls: --yes pre-decides the skill install
// (SkipSkillInstall=false) and no prompt is read.
func TestUpdateYes_AutoInstallsWithoutPrompt(t *testing.T) {
	_, root, out, _ := newUpdateEnv(t)
	setVersion(t, "v1.0.0")
	stubClient(t, &fakeClient{rel: relFixture()})
	withStdinTerminal(t, true)
	yesCalls := stubReadYesNo(t, true)
	calls := stubRun(t)

	stdout, err := execute(t, root, out, "update", "--yes", "--format", "json")
	require.NoError(t, err)

	require.Len(t, *calls, 1)
	assert.False(t, (*calls)[0].SkipSkillInstall, "--yes must not skip the skill install")
	assert.True(t, (*calls)[0].Yes)
	assert.Equal(t, 0, *yesCalls, "no prompt when --yes is given")
	assert.Contains(t, stdout, `"skill_installed"`)
	assert.Contains(t, stdout, `"skill_version": "v1.2.3"`)
	assert.NotContains(t, stdout, "Install the refreshed skill bundle")
	assert.Contains(t, stdout, `"skill_hint": ""`, "no hint when the install ran")
}

// TestUpdatePrompt_Accepted: on an interactive non-agent terminal the user
// is prompted; accepting proceeds with the skill install.
func TestUpdatePrompt_Accepted(t *testing.T) {
	_, root, out, _ := newUpdateEnv(t)
	setVersion(t, "v1.0.0")
	stubClient(t, &fakeClient{rel: relFixture()})
	withStdinTerminal(t, true)
	yesCalls := stubReadYesNo(t, true)
	calls := stubRun(t)

	stdout, err := execute(t, root, out, "update", "--format", "json")
	require.NoError(t, err)

	assert.Equal(t, 1, *yesCalls)
	require.Len(t, *calls, 1)
	assert.False(t, (*calls)[0].SkipSkillInstall)
	assert.Contains(t, stdout, "Install the refreshed skill bundle? [Y/n] ")
	assert.Contains(t, stdout, `"skill_installed"`)
}

// TestUpdatePrompt_Declined: declining at the prompt skips the skill
// install, so Run fills skill_hint instead.
func TestUpdatePrompt_Declined(t *testing.T) {
	_, root, out, _ := newUpdateEnv(t)
	setVersion(t, "v1.0.0")
	stubClient(t, &fakeClient{rel: relFixture()})
	withStdinTerminal(t, true)
	stubReadYesNo(t, false)
	calls := stubRun(t)

	stdout, err := execute(t, root, out, "update", "--format", "json")
	require.NoError(t, err)

	require.Len(t, *calls, 1)
	assert.True(t, (*calls)[0].SkipSkillInstall)
	assert.Contains(t, stdout, skipHint)
	// The row always carries the key; skipped installs render it empty.
	assert.Contains(t, stdout, `"skill_installed": null`)
	assert.Contains(t, stdout, `"skill_version": ""`)
}

// TestIsYes_DefaultIsYes pins the [Y/n] prompt semantics: only an explicit
// n/no declines; empty (just Enter) and anything else accepts.
func TestIsYes_DefaultIsYes(t *testing.T) {
	yes := []string{"", "y", "Y", "yes", "YES", "junk"}
	for _, in := range yes {
		assert.True(t, isYes(in), "%q should accept", in)
	}
	no := []string{"n", "N", "no", "NO"}
	for _, in := range no {
		assert.False(t, isYes(in), "%q should decline", in)
	}
}

// TestUpdateTable_CompactSkillPaths: table view drops the skill_installed
// column (a path list in one cell blows the table out horizontally) and
// lists the installed paths as one line each after the table instead.
// JSON/TOON keep the field.
func TestUpdateTable_CompactSkillPaths(t *testing.T) {
	_, root, out, _ := newUpdateEnv(t)
	setVersion(t, "v1.0.0")
	stubClient(t, &fakeClient{rel: relFixture()})
	calls := stubRun(t)

	stdout, err := execute(t, root, out, "update", "--yes", "--format", "table")
	require.NoError(t, err)

	require.Len(t, *calls, 1)
	assert.NotContains(t, stdout, "SKILL_INSTALLED", "no path-list column in table view")
	assert.Contains(t, stdout, "CURRENT_VERSION")
	assert.Contains(t, stdout, "SKILL_VERSION")
	assert.Contains(t, stdout, "installed everything-cli -> /home/u/.claude/skills/everything-cli")
}

// TestUpdatePrompt_NotReadWhenNonTTYOrAgent: when stdin is not a terminal,
// or an agent harness is detected, readYesNo must never be called and the
// install is skipped (hint path).
func TestUpdatePrompt_NotReadWhenNonTTYOrAgent(t *testing.T) {
	t.Run("non-TTY", func(t *testing.T) {
		_, root, out, _ := newUpdateEnv(t)
		setVersion(t, "v1.0.0")
		stubClient(t, &fakeClient{rel: relFixture()})
		// StdinIsTerminal stays pinned false by TestMain; a stub that would
		// record proves it is never consulted.
		calls := stubRun(t)
		_ = stubReadYesNo(t, false)

		stdout, err := execute(t, root, out, "update", "--format", "json")
		require.NoError(t, err)

		require.Len(t, *calls, 1)
		assert.True(t, (*calls)[0].SkipSkillInstall)
		assert.NotContains(t, stdout, "Install the refreshed skill bundle")
		assert.Contains(t, stdout, skipHint)
	})

	t.Run("agent-harness", func(t *testing.T) {
		_, root, out, _ := newUpdateEnv(t)
		setVersion(t, "v1.0.0")
		stubClient(t, &fakeClient{rel: relFixture()})
		withStdinTerminal(t, true)
		withAgent(t, true)
		calls := stubRun(t)
		yesCalls := stubReadYesNo(t, true)

		_, err := execute(t, root, out, "update", "--format", "json")
		require.NoError(t, err)

		require.Len(t, *calls, 1)
		assert.True(t, (*calls)[0].SkipSkillInstall)
		assert.Equal(t, 0, *yesCalls, "prompt must not be read in agent mode")
	})
}

// TestUpdateUpToDate_ExitsZeroWithResult: the real Run (via the default
// runUpdate seam) reports an equal version as ErrUpToDate; the command maps
// it to exit 0 with the comparison fields still rendered.
func TestUpdateUpToDate_ExitsZeroWithResult(t *testing.T) {
	_, root, out, _ := newUpdateEnv(t)
	setVersion(t, "v1.2.3")
	fc := &fakeClient{rel: relFixture()}
	stubClient(t, fc)

	stdout, err := execute(t, root, out, "update", "--yes", "--format", "json")
	require.NoError(t, err)

	assert.Contains(t, stdout, `"updated": false`)
	assert.Contains(t, stdout, `"update_available": false`)
	assert.Contains(t, stdout, `"latest_version": "v1.2.3"`)
	assert.Empty(t, fc.downloads, "up-to-date must not download anything")
}

// TestUpdateNoReleases_Error surfaces the client's no-releases sentinel.
func TestUpdateNoReleases_Error(t *testing.T) {
	_, root, out, _ := newUpdateEnv(t)
	setVersion(t, "v1.0.0")
	stubClient(t, &fakeClient{latestErr: updateapi.ErrNoReleases})

	_, err := execute(t, root, out, "update", "--yes", "--format", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no releases published yet")
}

// TestUpdateRateLimited_HintsToken: rate limiting errors carry the
// GITHUB_TOKEN hint.
func TestUpdateRateLimited_HintsToken(t *testing.T) {
	_, root, out, _ := newUpdateEnv(t)
	setVersion(t, "v1.0.0")
	stubClient(t, &fakeClient{latestErr: updateapi.ErrRateLimited})

	_, err := execute(t, root, out, "update", "--check", "--format", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rate limited by GitHub")
	assert.Contains(t, err.Error(), "GITHUB_TOKEN")
}

// TestUpdateAgentFilter_PassedThrough: --agent lands in Options.AgentFilter.
func TestUpdateAgentFilter_PassedThrough(t *testing.T) {
	_, root, out, _ := newUpdateEnv(t)
	setVersion(t, "v1.0.0")
	stubClient(t, &fakeClient{rel: relFixture()})
	calls := stubRun(t)

	_, err := execute(t, root, out, "update", "--yes", "--agent", "claude-code", "--format", "json")
	require.NoError(t, err)

	require.Len(t, *calls, 1)
	assert.Equal(t, "claude-code", (*calls)[0].AgentFilter)
	assert.False(t, (*calls)[0].SkipSkillInstall)
}

// TestUpdateRunPipeline_ProceedsToDownload: with the real runUpdate seam and
// --yes, an available update reaches the download phase (the fake client
// refuses, proving Run was entered past the version comparison).
func TestUpdateRunPipeline_ProceedsToDownload(t *testing.T) {
	_, root, out, _ := newUpdateEnv(t)
	setVersion(t, "v1.0.0")
	fc := &fakeClient{rel: relFixture()}
	stubClient(t, fc)

	_, err := execute(t, root, out, "update", "--yes", "--format", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fakeClient: Download must not be called")
}
