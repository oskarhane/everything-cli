package skill

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/afero"
)

// versionLineRe matches the frontmatter `version:` line in an installed
// SKILL.md. Tolerates leading whitespace and arbitrary trailing whitespace
// after the value.
var versionLineRe = regexp.MustCompile(`(?m)^[ \t]*version:[ \t]*([^\r\n]*?)[ \t]*$`)

// frontmatterRe matches the leading YAML frontmatter block of a SKILL.md
// body. Captures the inner body so injection can replace or append the
// `version:` line within it.
var frontmatterRe = regexp.MustCompile(`(?s)\A---\r?\n(.*?)\r?\n---(\r?\n|\z)`)

// Install copies the embedded Bundle into each target agent's skills
// directory under `<expanded SkillsDir>/google-cli/`. The SKILL.md
// frontmatter `version:` line is rewritten (or inserted) to `version`;
// references are copied verbatim. Each target is installed clean-slate
// (prior contents removed first) so deleted reference files don't linger.
//
// agentFilter semantics:
//   - "" — install to every detected agent. Returns ErrNoAgentsDetected
//     when none are detected.
//   - non-empty — case-insensitive lookup in AGENTS. Unknown returns
//     ErrUnknownAgent; known-but-undetected returns ErrAgentNotDetected.
//
// InstallResult describes one completed bundle install.
type InstallResult struct {
	Agent string // agent id the bundle was written to
	Path  string // the installed <skills>/google-cli directory
}

func Install(filesystem afero.Fs, version, agentFilter string) ([]InstallResult, error) {
	targets, err := targets(filesystem, agentFilter)
	if err != nil {
		return nil, err
	}
	if len(targets) == 0 {
		return nil, ErrNoAgentsDetected
	}

	results := make([]InstallResult, 0, len(targets))
	for _, a := range targets {
		// A named target need not be detected; installing into an
		// undetected agent is an error (Remove tolerates it instead).
		if agentFilter != "" {
			dp, ok := a.DetectPath()
			if !ok {
				return nil, fmt.Errorf("%w: %s (cannot resolve HOME)", ErrAgentNotDetected, a.Name)
			}
			exists, _ := afero.DirExists(filesystem, dp)
			if !exists {
				return nil, fmt.Errorf("%w: %s", ErrAgentNotDetected, a.Name)
			}
		}

		skillsRoot, ok := a.SkillsPath()
		if !ok {
			return nil, fmt.Errorf("skill: cannot resolve skills path for %s", a.Name)
		}
		dst := filepath.Join(skillsRoot, SkillName)

		// Clean any prior install so removed reference files don't linger.
		if rerr := RemoveDir(filesystem, dst); rerr != nil {
			return nil, fmt.Errorf("skill: cleaning %s: %w", dst, rerr)
		}
		if cerr := copyBundleWithVersion(filesystem, dst, Bundle, version); cerr != nil {
			return nil, fmt.Errorf("skill: writing %s: %w", dst, cerr)
		}
		results = append(results, InstallResult{Agent: a.Name, Path: dst})
	}
	return results, nil
}

// targets resolves the shared agent filter for Install and Remove: "" means
// every detected agent (possibly none); a non-empty filter is a
// case-insensitive lookup whose unknown values are ErrUnknownAgent.
// Stricter install-only semantics (non-empty detection) live in Install.
func targets(filesystem afero.Fs, agentFilter string) ([]Agent, error) {
	if agentFilter == "" {
		return DetectAgents(filesystem), nil
	}
	a := FindAgent(agentFilter)
	if a == nil {
		return nil, fmt.Errorf("%w: %q", ErrUnknownAgent, agentFilter)
	}
	return []Agent{*a}, nil
}

// copyBundleWithVersion copies the bundle FS into dstDir via CopyBundle,
// rewriting the SKILL.md frontmatter `version:` line to version (or
// inserting one when upstream has none). References are copied verbatim.
func copyBundleWithVersion(dst afero.Fs, dstDir string, bundle fs.FS, version string) error {
	return CopyBundle(dst, dstDir, bundle, func(p string, data []byte) []byte {
		if p == "SKILL.md" {
			return injectVersion(data, version)
		}
		return data
	})
}

// injectVersion rewrites the SKILL.md frontmatter `version:` line to the
// supplied value, or inserts one immediately before the closing `---`
// fence when absent. An empty `version` is a no-op. If `data` has no
// frontmatter block, it is returned unchanged.
func injectVersion(data []byte, version string) []byte {
	if version == "" {
		return data
	}
	m := frontmatterRe.FindSubmatchIndex(data)
	if m == nil {
		return data
	}
	innerStart, innerEnd := m[2], m[3]
	inner := string(data[innerStart:innerEnd])

	newLine := "version: " + version
	var newInner string
	if versionLineRe.MatchString(inner) {
		newInner = versionLineRe.ReplaceAllLiteralString(inner, newLine)
	} else {
		trimmed := strings.TrimRight(inner, "\r\n")
		if trimmed == "" {
			newInner = newLine
		} else {
			newInner = trimmed + "\n" + newLine
		}
	}

	var out strings.Builder
	out.Grow(len(data) + len(newLine))
	out.Write(data[:innerStart])
	out.WriteString(newInner)
	out.Write(data[innerEnd:])
	return []byte(out.String())
}
