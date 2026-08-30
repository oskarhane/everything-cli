package skill

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/afero"
)

// Remove deletes the installed bundle from each target agent's skills
// directory. Idempotent: missing install dirs return nil.
//
// agentFilter semantics:
//   - "" — remove from every detected agent (no error if none are
//     detected; that is a no-op returning an empty slice).
//   - non-empty — case-insensitive lookup; unknown returns ErrUnknownAgent.
//
// Removing from an undetected-but-known agent is a no-op (the install dir
// can't exist if the agent dir doesn't).
func Remove(filesystem afero.Fs, agentFilter string) ([]Agent, error) {
	var targets []Agent
	if agentFilter == "" {
		targets = DetectAgents(filesystem)
	} else {
		a := FindAgent(agentFilter)
		if a == nil {
			return nil, fmt.Errorf("%w: %q", ErrUnknownAgent, agentFilter)
		}
		targets = []Agent{*a}
	}

	for _, a := range targets {
		skillsRoot, ok := a.SkillsPath()
		if !ok {
			continue
		}
		dst := filepath.Join(skillsRoot, SkillName)
		if rerr := RemoveDir(filesystem, dst); rerr != nil {
			return nil, fmt.Errorf("skill: removing %s: %w", dst, rerr)
		}
	}
	return targets, nil
}
