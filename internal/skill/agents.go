package skill

import (
	"strings"

	"github.com/spf13/afero"
)

// Agent describes a supported AI agent: where its install marker lives and
// where its skill bundles go. DetectDir / SkillsDir are stored in their
// unexpanded form (with `~` and `$XDG_CONFIG_HOME`); call DetectPath /
// SkillsPath to resolve.
type Agent struct {
	Name        string // canonical lowercase id, e.g. "claude-code"
	DisplayName string // human-readable, e.g. "Claude Code"
	DetectDir   string // unexpanded path used to detect agent presence
	SkillsDir   string // unexpanded path where skill bundles are placed
}

// AGENTS is the supported agent catalog. Order is preserved for stable
// list output. Ported from neo4j-cli common/skill/agents.go (minus the
// MCP-only claude-desktop entry).
var AGENTS = []Agent{
	{Name: "claude-code", DisplayName: "Claude Code", DetectDir: "~/.claude", SkillsDir: "~/.claude/skills"},
	{Name: "cursor", DisplayName: "Cursor", DetectDir: "~/.cursor", SkillsDir: "~/.cursor/skills"},
	{Name: "windsurf", DisplayName: "Windsurf", DetectDir: "~/.codeium/windsurf", SkillsDir: "~/.codeium/windsurf/skills"},
	{Name: "copilot", DisplayName: "Copilot", DetectDir: "~/.copilot", SkillsDir: "~/.copilot/skills"},
	{Name: "antigravity", DisplayName: "Antigravity", DetectDir: "~/.gemini/antigravity", SkillsDir: "~/.gemini/antigravity/skills"},
	{Name: "gemini-cli", DisplayName: "Gemini CLI", DetectDir: "~/.gemini", SkillsDir: "~/.gemini/skills"},
	{Name: "cline", DisplayName: "Cline", DetectDir: "~/.cline", SkillsDir: "~/.agents/skills"},
	{Name: "codex", DisplayName: "Codex", DetectDir: "~/.codex", SkillsDir: "~/.codex/skills"},
	{Name: "pi", DisplayName: "Pi", DetectDir: "~/.pi/agent", SkillsDir: "~/.pi/agent/skills"},
	{Name: "opencode", DisplayName: "OpenCode", DetectDir: "$XDG_CONFIG_HOME/opencode", SkillsDir: "$XDG_CONFIG_HOME/opencode/skills"},
	{Name: "junie", DisplayName: "Junie", DetectDir: "~/.junie", SkillsDir: "~/.junie/skills"},
}

// DetectPath returns the expanded DetectDir and ok=true if expansion
// succeeded. ok=false signals that no $HOME is available, in which case
// the agent should be treated as not-detected.
func (a Agent) DetectPath() (string, bool) {
	return expandPath(a.DetectDir)
}

// SkillsPath returns the expanded SkillsDir and ok=true if expansion
// succeeded. ok=false signals that no $HOME is available; the guard lives
// here so call sites cannot install into a relative path.
func (a Agent) SkillsPath() (string, bool) {
	return expandPath(a.SkillsDir)
}

// FindAgent looks up an agent by name, case-insensitive. Returns nil if no
// match. The returned pointer is stable (points into AGENTS).
func FindAgent(name string) *Agent {
	lower := strings.ToLower(name)
	for i := range AGENTS {
		if AGENTS[i].Name == lower {
			return &AGENTS[i]
		}
	}
	return nil
}

// DetectAgents returns the agents whose DetectDir exists on the given
// filesystem, in AGENTS order. Hermetic-friendly: pass afero.NewMemMapFs
// in tests.
func DetectAgents(fs afero.Fs) []Agent {
	out := make([]Agent, 0, len(AGENTS))
	for _, a := range AGENTS {
		p, ok := a.DetectPath()
		if !ok {
			continue
		}
		exists, err := afero.DirExists(fs, p)
		if err != nil || !exists {
			continue
		}
		out = append(out, a)
	}
	return out
}
