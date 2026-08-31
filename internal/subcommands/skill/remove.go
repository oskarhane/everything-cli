package skill

import (
	"fmt"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/skill"
	"github.com/spf13/cobra"
)

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
			results, err := skill.Remove(cfg.Fs, agent)
			if err != nil {
				return wrapAgentFilterError(err)
			}
			out := cmd.OutOrStdout()
			removedAny := false
			for _, r := range results {
				if r.Removed {
					removedAny = true
					_, _ = fmt.Fprintf(out, "removed %s from %s\n", skill.SkillName, r.Path)
				} else {
					_, _ = fmt.Fprintf(out, "no %s install in %s\n", skill.SkillName, r.Path)
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
