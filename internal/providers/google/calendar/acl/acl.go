// Package acl builds the `calendar acl` command subtree.
package acl

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/subcommands/calendar/service"
)

// NewCmd returns the `calendar acl` parent with every acl leaf attached.
// Each leaf lives in its own file: list.go, add.go, remove.go.
func NewCmd(cfg *app.Config, newSvc service.Dialer[service.CalendarService]) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "acl",
		Short: "Manage calendar sharing rules",
	}
	cmd.AddCommand(newListCmd(cfg, newSvc))
	cmd.AddCommand(newAddCmd(cfg, newSvc))
	cmd.AddCommand(newRemoveCmd(cfg, newSvc))
	return cmd
}
