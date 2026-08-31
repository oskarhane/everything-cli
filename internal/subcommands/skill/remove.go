package skill

import (
	"fmt"
	"path/filepath"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/skill"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

// removeTarget is one agent scoped for removal, with its resolved install
// dir and whether the bundle was present before the removal ran.
type removeTarget struct {
	agent  skill.Agent
	path   string
	exists bool
}

// newRemoveCmd builds skill remove: delete the installed google-cli skill
// bundle from each target agent's skills directory. Idempotent.
func newRemoveCmd(cfg *app.Config) *cobra.Command {
	var agent string
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove the google-cli skill bundle from AI agents' skills dirs",
		Long: "Remove the installed google-cli skill bundle from every detected AI " +
			"agent, or only the agent named by --agent. Removal is idempotent: " +
			"agents without an installed bundle are reported as no-ops, not errors.",
		Example: `# Remove the bundle from every agent it was installed into
google-cli skill remove

# Remove only from Claude Code's skills directory
google-cli skill remove --agent claude-code

# Re-install the bundle afterwards if it was removed by mistake
google-cli skill install`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			targets, err := preRemoveTargets(cfg, agent)
			if err != nil {
				return err
			}
			if _, err := skill.Remove(cfg.Fs, agent); err != nil {
				return wrapAgentFilterError(err)
			}
			out := cmd.OutOrStdout()
			removedAny := false
			for _, t := range targets {
				if t.exists {
					removedAny = true
					_, _ = fmt.Fprintf(out, "removed %s from %s\n", skill.SkillName, t.path)
				} else {
					_, _ = fmt.Fprintf(out, "no %s install in %s\n", skill.SkillName, t.path)
				}
			}
			if removedAny {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(),
					"run 'google-cli skill install' to reinstall\n")
			}
			return nil
		},
	}
	addAgentFlag(cmd, &agent)
	return cmd
}

// preRemoveTargets resolves the same targets skill.Remove will act on and
// records whether each install dir currently exists, so the output can
// distinguish a real removal from an idempotent no-op.
func preRemoveTargets(cfg *app.Config, agentFilter string) ([]removeTarget, error) {
	var targets []skill.Agent
	if agentFilter == "" {
		targets = skill.DetectAgents(cfg.Fs)
	} else {
		a := skill.FindAgent(agentFilter)
		if a == nil {
			return nil, wrapAgentFilterError(
				fmt.Errorf("%w: %q", skill.ErrUnknownAgent, agentFilter))
		}
		targets = []skill.Agent{*a}
	}

	out := make([]removeTarget, 0, len(targets))
	for _, a := range targets {
		root, ok := a.SkillsPath()
		if !ok {
			continue
		}
		dst := filepath.Join(root, skill.SkillName)
		exists, err := afero.DirExists(cfg.Fs, dst)
		if err != nil {
			return nil, fmt.Errorf("skill: inspecting %s: %w", dst, err)
		}
		out = append(out, removeTarget{path: dst, exists: exists})
	}
	return out, nil
}
