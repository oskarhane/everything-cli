package skill

import (
	"fmt"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/skill"
	"github.com/spf13/cobra"
)

// newInstallCmd builds skill install: copy the embedded google-cli skill
// bundle into each target agent's skills directory.
func newInstallCmd(cfg *app.Config) *cobra.Command {
	var agent string
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install the google-cli skill bundle into AI agents' skills dirs",
		Long: "Install the embedded google-cli skill bundle (SKILL.md plus references) " +
			"into <skills dir>/google-cli/ for every detected AI agent, or only the " +
			"agent named by --agent. Each install is clean-slate: prior contents of " +
			"the google-cli skill dir are replaced. Detection is by the agent's " +
			"config directory existing on disk (e.g. ~/.claude).",
		Example: `# Install into every detected agent
google-cli skill install

# Install only into Claude Code's skills directory
google-cli skill install --agent claude-code

# Re-install after a google-cli upgrade to refresh the bundled docs
google-cli skill install`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			installed, err := skill.Install(cfg.Fs, app.Version, agent)
			if err != nil {
				return wrapAgentFilterError(err)
			}
			for _, r := range installed {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "installed %s -> %s\n",
					skill.SkillName, r.Path)
			}
			return nil
		},
	}
	addAgentFlag(cmd, &agent)
	return cmd
}
