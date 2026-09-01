// Package skill implements the everything-cli skill subcommands: installing,
// removing, and printing the embedded agent-skill bundle that teaches AI
// agents how to use this CLI.
package skill

import (
	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/spf13/cobra"
)

// NewCmd builds the skill parent command: install/remove/print of the
// embedded everything-cli agent-skill bundle.
func NewCmd(cfg *app.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skill",
		Short: "Manage the everything-cli agent-skill bundle for AI agents",
		Long: "Manage the everything-cli agent-skill bundle: install it into the skills " +
			"directory of detected AI agents, remove it again, or print its raw " +
			"markdown. These commands are local-only — they never touch Google.",
	}

	cmd.AddCommand(newInstallCmd(cfg))
	cmd.AddCommand(newRemoveCmd(cfg))
	cmd.AddCommand(newPrintCmd(cfg))

	return cmd
}
