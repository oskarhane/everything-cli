package skill

import (
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedDetected installs the DetectDir for each named agent so Install sees
// them as present. opencode requires XDG seeding which callers handle.
func seedAgents(t *testing.T, fsys afero.Fs, names ...string) {
	t.Helper()
	for _, name := range names {
		a := FindAgent(name)
		require.NotNil(t, a, "test seed references unknown agent %q", name)
		p, ok := a.DetectPath()
		require.True(t, ok)
		mkdir(t, fsys, p)
	}
}

// destFor is the install destination of the bundle for an agent.
func destFor(a Agent, home string) string {
	p, _ := a.SkillsPath()
	return filepath.Join(p, SkillName)
}

// TestInstallAllDetected: with no filter, Install writes the bundle into
// every detected agent and returns them in catalog order.
func TestInstallAllDetected(t *testing.T) {
	home := t.TempDir()
	fsys := newTestFS(t, home)
	seedAgents(t, fsys, "claude-code", "codex")

	installed, err := Install(fsys, "v1.2.3", "")
	require.NoError(t, err)

	names := make([]string, 0, len(installed))
	for _, r := range installed {
		names = append(names, r.Agent)
		assert.Equal(t, destFor(*FindAgent(r.Agent), home), r.Path, "Path is the install dir")
	}
	assert.Equal(t, []string{"claude-code", "codex"}, names)

	for _, name := range names {
		skillMD := filepath.Join(destFor(*FindAgent(name), home), "SKILL.md")
		exists, ferr := afero.Exists(fsys, skillMD)
		require.NoError(t, ferr)
		assert.True(t, exists, skillMD)
	}
}

// TestInstallSingleAgent: a non-empty filter installs only that agent,
// case-insensitively.
func TestInstallSingleAgent(t *testing.T) {
	home := t.TempDir()
	fsys := newTestFS(t, home)
	seedAgents(t, fsys, "claude-code", "codex")

	installed, err := Install(fsys, "v1", "CLAUDE-CODE")
	require.NoError(t, err)
	require.Len(t, installed, 1)
	assert.Equal(t, "claude-code", installed[0].Agent)
	assert.Equal(t, destFor(*FindAgent("claude-code"), home), installed[0].Path)

	exists, err := afero.Exists(fsys, filepath.Join(destFor(*FindAgent("codex"), home), "SKILL.md"))
	require.NoError(t, err)
	assert.False(t, exists, "unfiltered agent must not receive the bundle")
}

// TestInstallErrors: unknown filter and zero detected agents are errors.
func TestInstallErrors(t *testing.T) {
	t.Run("unknown agent", func(t *testing.T) {
		home := t.TempDir()
		fsys := newTestFS(t, home)

		_, err := Install(fsys, "v1", "bogus")
		require.ErrorIs(t, err, ErrUnknownAgent)
	})

	t.Run("no agents detected", func(t *testing.T) {
		home := t.TempDir()
		fsys := newTestFS(t, home)

		_, err := Install(fsys, "v1", "")
		require.ErrorIs(t, err, ErrNoAgentsDetected)
	})

	t.Run("known but not detected", func(t *testing.T) {
		home := t.TempDir()
		fsys := newTestFS(t, home)

		_, err := Install(fsys, "v1", "claude-code")
		require.ErrorIs(t, err, ErrAgentNotDetected)
	})
}

// TestInstallCleanSlate: a pre-seeded stale file in the destination (e.g.
// a reference removed upstream) is gone after install.
func TestInstallCleanSlate(t *testing.T) {
	home := t.TempDir()
	fsys := newTestFS(t, home)
	seedAgents(t, fsys, "claude-code")

	dest := destFor(*FindAgent("claude-code"), home)
	stale := filepath.Join(dest, "references", "old.md")
	require.NoError(t, fsys.MkdirAll(filepath.Dir(stale), 0o755))
	require.NoError(t, afero.WriteFile(fsys, stale, []byte("stale"), 0o600))

	_, err := Install(fsys, "v1", "")
	require.NoError(t, err)

	exists, err := afero.Exists(fsys, stale)
	require.NoError(t, err)
	assert.False(t, exists, "stale file must be gone after clean-slate install")

	exists, err = afero.Exists(fsys, filepath.Join(dest, "SKILL.md"))
	require.NoError(t, err)
	assert.True(t, exists)

	exists, err = afero.Exists(fsys, filepath.Join(dest, "references", "google.md"))
	require.NoError(t, err)
	assert.True(t, exists)
}

// TestInstallRewritesVersionLine: the frontmatter `version:` line carries
// the install version, not the upstream placeholder.
func TestInstallRewritesVersionLine(t *testing.T) {
	home := t.TempDir()
	fsys := newTestFS(t, home)
	seedAgents(t, fsys, "claude-code")

	_, err := Install(fsys, "v9.9.9", "")
	require.NoError(t, err)

	data, err := afero.ReadFile(fsys, filepath.Join(destFor(*FindAgent("claude-code"), home), "SKILL.md"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "version: v9.9.9")
	assert.NotContains(t, string(data), "version: dev")
}

// TestInstallFileModes: directories 0755, files 0600 (MemMapFs applies no
// umask, so the exact modes are assertable).
func TestInstallFileModes(t *testing.T) {
	home := t.TempDir()
	fsys := newTestFS(t, home)
	seedAgents(t, fsys, "claude-code")

	_, err := Install(fsys, "v1", "")
	require.NoError(t, err)

	dest := destFor(*FindAgent("claude-code"), home)

	info, err := fsys.Stat(dest)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
	assert.Equal(t, fs.FileMode(0o755).Perm(), info.Mode().Perm(), "bundle dir mode")

	info, err = fsys.Stat(filepath.Join(dest, "references"))
	require.NoError(t, err)
	assert.Equal(t, fs.FileMode(0o755).Perm(), info.Mode().Perm(), "references dir mode")

	info, err = fsys.Stat(filepath.Join(dest, "SKILL.md"))
	require.NoError(t, err)
	assert.Equal(t, fs.FileMode(0o600).Perm(), info.Mode().Perm(), "SKILL.md mode")

	info, err = fsys.Stat(filepath.Join(dest, "references", "google.md"))
	require.NoError(t, err)
	assert.Equal(t, fs.FileMode(0o600).Perm(), info.Mode().Perm(), "reference file mode")
}

// TestInjectVersion: rewrites an existing `version:` line and inserts one
// before the closing fence when the frontmatter has none.
func TestInjectVersion(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		version string
		want    string
	}{
		{
			name:    "rewrites existing line",
			in:      "---\nname: everything-cli\nversion: dev\n---\nbody\n",
			version: "v1.0.0",
			want:    "---\nname: everything-cli\nversion: v1.0.0\n---\nbody\n",
		},
		{
			name:    "inserts when absent",
			in:      "---\nname: everything-cli\ndescription: d\n---\nbody\n",
			version: "v1.0.0",
			want:    "---\nname: everything-cli\ndescription: d\nversion: v1.0.0\n---\nbody\n",
		},
		{
			name:    "no frontmatter unchanged",
			in:      "# just markdown\n",
			version: "v1.0.0",
			want:    "# just markdown\n",
		},
		{
			name:    "empty version is a no-op",
			in:      "---\nname: everything-cli\nversion: dev\n---\nbody\n",
			version: "",
			want:    "---\nname: everything-cli\nversion: dev\n---\nbody\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, string(injectVersion([]byte(tt.in), tt.version)))
		})
	}
}

// TestInstallWrittenBundleIsComplete: the written bundle carries the full
// reference set and the source frontmatter identity.
func TestInstallWrittenBundleIsComplete(t *testing.T) {
	home := t.TempDir()
	fsys := newTestFS(t, home)
	seedAgents(t, fsys, "claude-code")

	_, err := Install(fsys, "v1", "")
	require.NoError(t, err)

	for _, p := range []string{
		filepath.Join(destFor(*FindAgent("claude-code"), home), "references", "google.md"),
		filepath.Join(destFor(*FindAgent("claude-code"), home), "references", "granola.md"),
		filepath.Join(destFor(*FindAgent("claude-code"), home), "references", "linear.md"),
	} {
		exists, ferr := afero.Exists(fsys, p)
		require.NoError(t, ferr, p)
		assert.True(t, exists, p)
	}

	data, rerr := afero.ReadFile(fsys, filepath.Join(destFor(*FindAgent("claude-code"), home), "SKILL.md"))
	require.NoError(t, rerr)
	assert.Contains(t, string(data), "name: everything-cli")
}

// TestParseVersionLineTolerances: ParseVersionLine reads a stamped version
// through the same matcher injectVersion uses. Relocated from
// internal/update's duplicate parser (node 13 consolidation).
func TestParseVersionLineTolerances(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "frontmatter", body: "---\nname: x\nversion: 1.2.3\n---\nbody", want: "1.2.3"},
		{name: "crlf", body: "---\r\nversion: 1.2.3\r\n---\r\n", want: "1.2.3"},
		{name: "trailing spaces", body: "version: 1.2.3   \n", want: "1.2.3"},
		{name: "indented", body: "  version: 0.9.0\n", want: "0.9.0"},
		{name: "absent", body: "---\nname: x\n---\n", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseVersionLine([]byte(tt.body))
			assert.Equal(t, tt.want != "", ok)
			assert.Equal(t, tt.want, got)
		})
	}
}
