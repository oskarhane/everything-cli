// Package event builds the `calendar event` command subtree: CRUD leaves
// plus recurring-event scope handling (--this-only / --all) and the
// accept/decline/tentative response leaves.
package event

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/google/calendar/service"
)

// NewCmd returns the `calendar event` parent with every leaf attached. Each
// leaf lives in its own file; respond.go builds the three response verbs.
func NewCmd(cfg *app.Config, newSvc service.Dialer[service.EventService]) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "event",
		Short: "Manage calendar events, including recurring ones",
	}
	cmd.AddCommand(newListCmd(cfg, newSvc))
	cmd.AddCommand(newGetCmd(cfg, newSvc))
	cmd.AddCommand(newInstancesCmd(cfg, newSvc))
	cmd.AddCommand(newCreateCmd(cfg, newSvc))
	cmd.AddCommand(newUpdateCmd(cfg, newSvc))
	cmd.AddCommand(newDeleteCmd(cfg, newSvc))
	cmd.AddCommand(newMoveCmd(cfg, newSvc))
	for _, verb := range []string{"accept", "decline", "tentative"} {
		cmd.AddCommand(newRespondCmd(cfg, newSvc, verb))
	}
	return cmd
}
