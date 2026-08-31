package update

import (
	"errors"
	"fmt"
	"strings"

	"github.com/oskarhane/google-cli/internal/skill"
	"github.com/spf13/cobra"
)

// addAgentFlag registers the shared --agent filter flag on the update leaf.
// Duplicated (not shared) with the skill package's copy on purpose: both
// packages keep their wiring private.
func addAgentFlag(cmd *cobra.Command, agent *string) {
	cmd.Flags().StringVar(agent, "agent", "",
		"Act only on this agent id (case-insensitive; empty = all detected agents)")
}

// wrapAgentFilterError maps internal/skill sentinel errors to user-facing
// messages. Unknown agents get the list of valid agent ids; zero detected
// agents get a hint about creating an agent dir or passing --agent.
func wrapAgentFilterError(err error) error {
	switch {
	case errors.Is(err, skill.ErrUnknownAgent):
		return fmt.Errorf("%w; valid agents: %s", err, validAgentNames())
	case errors.Is(err, skill.ErrNoAgentsDetected):
		return fmt.Errorf(
			"%w — create one of the agent directories on disk (e.g. ~/.claude) or pass --agent <id>",
			err)
	case errors.Is(err, skill.ErrAgentNotDetected):
		return fmt.Errorf("%w — its directory was not found on this machine", err)
	default:
		return err
	}
}

// validAgentNames returns the supported agent ids as a comma-separated
// string, in catalog order.
func validAgentNames() string {
	names := make([]string, 0, len(skill.AGENTS))
	for _, a := range skill.AGENTS {
		names = append(names, a.Name)
	}
	return strings.Join(names, ", ")
}
