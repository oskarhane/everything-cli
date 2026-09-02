package skill

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/afero"
)

// RemoveResult describes the outcome of one removal target.
type RemoveResult struct {
	Agent   string // agent id whose skills dir was acted on
	Path    string // the <skills>/everything-cli directory that was inspected
	Removed bool   // true when the bundle dir existed and was removed
}

// Remove deletes the installed bundle from each target agent's skills
// directory. Idempotent: missing install dirs are reported with
// Removed=false, not errors.
//
// agentFilter semantics:
//   - "" — remove from every detected agent (no error if none are
//     detected; that is a no-op returning an empty slice).
//   - non-empty — case-insensitive lookup; unknown returns ErrUnknownAgent.
//
// Removing from an undetected-but-known agent is a no-op (the install dir
// can't exist if the agent dir doesn't).
func Remove(filesystem afero.Fs, agentFilter string) ([]RemoveResult, error) {
	targets, err := targets(filesystem, agentFilter)
	if err != nil {
		return nil, err
	}

	results := make([]RemoveResult, 0, len(targets))
	for _, a := range targets {
		skillsRoot, ok := a.SkillsPath()
		if !ok {
			continue
		}
		dst := filepath.Join(skillsRoot, SkillName)
		exists, derr := afero.DirExists(filesystem, dst)
		if derr != nil {
			return nil, fmt.Errorf("skill: inspecting %s: %w", dst, derr)
		}
		if rerr := RemoveDir(filesystem, dst); rerr != nil {
			return nil, fmt.Errorf("skill: removing %s: %w", dst, rerr)
		}
		results = append(results, RemoveResult{Agent: a.Name, Path: dst, Removed: exists})
	}
	return results, nil
}
