// Package acl builds the `calendar acl` command subtree.
package acl

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/subcommands/calendar/service"
)

// serviceFunc builds the Calendar service a leaf's RunE uses. The calendar
// parent injects the real dialer; tests inject fakes so no leaf ever touches
// the network.
type serviceFunc func(context.Context) (service.CalendarService, error)

// NewCmd returns the `calendar acl` parent with every acl leaf attached.
// Each leaf lives in its own file: list.go, add.go, remove.go.
func NewCmd(cfg *app.Config, newSvc serviceFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "acl",
		Short: "Manage calendar sharing rules",
	}
	cmd.AddCommand(newListCmd(cfg, newSvc))
	cmd.AddCommand(newAddCmd(cfg, newSvc))
	cmd.AddCommand(newRemoveCmd(cfg, newSvc))
	return cmd
}
