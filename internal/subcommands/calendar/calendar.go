// Package calendar builds the `calendar` command tree.
package calendar

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/subcommands/calendar/acl"
	"github.com/oskarhane/google-cli/internal/subcommands/calendar/calendarlist"
	"github.com/oskarhane/google-cli/internal/subcommands/calendar/event"
	"github.com/oskarhane/google-cli/internal/subcommands/calendar/freebusy"
	"github.com/oskarhane/google-cli/internal/subcommands/calendar/service"
)

// NewCmd returns the `calendar` parent command with its subtrees attached.
// The calendar CRUD leaves hang directly off this parent (the resource IS
// the calendar), so calendarlist.NewLeaves supplies them; acl is a subgroup.
// Leaf bodies live in their own files under calendarlist/ and acl/.
func NewCmd(cfg *app.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "calendar",
		Short: "Manage calendars, their sharing rules, events, and free/busy",
	}
	newSvc := func(ctx context.Context) (service.CalendarService, error) {
		return dial(ctx, cfg)
	}
	cmd.AddCommand(calendarlist.NewLeaves(cfg, newSvc)...)
	cmd.AddCommand(acl.NewCmd(cfg, newSvc))
	cmd.AddCommand(event.NewCmd(cfg, func(ctx context.Context) (service.EventService, error) {
		return service.AsEventService(dial(ctx, cfg))
	}))
	cmd.AddCommand(freebusy.NewCmd(cfg, func(ctx context.Context) (service.FreeBusyService, error) {
		return service.AsFreeBusyService(dial(ctx, cfg))
	}))
	return cmd
}
