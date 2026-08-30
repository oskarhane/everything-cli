package skill

import (
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAGENTS: the catalog carries exactly the 11 skill-capable agents with
// their unexpanded detect/skills dirs (port of neo4j-cli common/skill).
func TestAGENTS(t *testing.T) {
	want := []struct {
		name, displayName, detect, skills string
	}{
		{"claude-code", "Claude Code", "~/.claude", "~/.claude/skills"},
		{"cursor", "Cursor", "~/.cursor", "~/.cursor/skills"},
		{"windsurf", "Windsurf", "~/.codeium/windsurf", "~/.codeium/windsurf/skills"},
		{"copilot", "Copilot", "~/.copilot", "~/.copilot/skills"},
		{"antigravity", "Antigravity", "~/.gemini/antigravity", "~/.gemini/antigravity/skills"},
		{"gemini-cli", "Gemini CLI", "~/.gemini", "~/.gemini/skills"},
		{"cline", "Cline", "~/.cline", "~/.agents/skills"},
		{"codex", "Codex", "~/.codex", "~/.codex/skills"},
		{"pi", "Pi", "~/.pi/agent", "~/.pi/agent/skills"},
		{"opencode", "OpenCode", "$XDG_CONFIG_HOME/opencode", "$XDG_CONFIG_HOME/opencode/skills"},
		{"junie", "Junie", "~/.junie", "~/.junie/skills"},
	}

	require.Len(t, AGENTS, len(want))
	for i, w := range want {
		a := AGENTS[i]
		assert.Equal(t, w.name, a.Name, "entry %d name", i)
		assert.Equal(t, w.displayName, a.DisplayName, "entry %d display name", i)
		assert.Equal(t, w.detect, a.DetectDir, "entry %d detect dir", i)
		assert.Equal(t, w.skills, a.SkillsDir, "entry %d skills dir", i)
	}
}

// TestFindAgent: case-insensitive lookup over the catalog, nil when
// unknown (claude-desktop is not in this catalog).
func TestFindAgent(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string // expected catalog Name, "" for nil
		wantNil bool
	}{
		{"exact", "claude-code", "claude-code", false},
		{"upper", "CLAUDE-CODE", "claude-code", false},
		{"mixed", "Gemini-CLI", "gemini-cli", false},
		{"unknown", "claude-desktop", "", true},
		{"bogus", "not-an-agent", "", true},
		{"empty", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FindAgent(tt.in)
			if tt.wantNil {
				assert.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			assert.Equal(t, tt.want, got.Name)
		})
	}
}

// TestDetectAgents: detection walks DetectDir presence on the passed FS,
// honours $XDG_CONFIG_HOME (with $HOME/.config fallback) for opencode, and
// skips agents whose DetectDir is missing.
func TestDetectAgents(t *testing.T) {
	tests := []struct {
		name string
		xdg  string
		seed []string // DetectDir paths to create, relative to home
		want []string
	}{
		{
			name: "single detected",
			seed: []string{".claude"},
			want: []string{"claude-code"},
		},
		{
			name: "several detected in catalog order",
			seed: []string{".junie", ".claude", ".codex"},
			want: []string{"claude-code", "codex", "junie"},
		},
		{
			name: "missing dirs skipped",
			seed: []string{},
			want: []string{},
		},
		{
			name: "opencode via XDG_CONFIG_HOME",
			xdg:  "/xdg-root",
			seed: []string{}, // seed XDG dir below, not under home
			want: []string{"opencode"},
		},
		{
			name: "opencode XDG fallback to HOME/.config",
			seed: []string{".config/opencode"},
			want: []string{"opencode"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("XDG_CONFIG_HOME", tt.xdg)
			fs := afero.NewMemMapFs()

			for _, s := range tt.seed {
				mkdir(t, fs, homePath(home, s))
			}
			if tt.xdg != "" {
				mkdir(t, fs, filepath.Join(tt.xdg, "opencode"))
			}

			detected := DetectAgents(fs)
			names := make([]string, 0, len(detected))
			for _, a := range detected {
				names = append(names, a.Name)
			}
			assert.Equal(t, tt.want, names)
		})
	}
}
