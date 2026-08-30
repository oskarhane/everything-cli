// Package calendarlist builds the calendar CRUD leaves that hang directly
// off the `calendar` parent: list, get, create, update, delete. The resource
// is the calendar itself, so unlike gmail's label subgroup there is no extra
// parent level — NewLeaves returns the leaf commands for the calendar parent
// to attach.
package calendarlist

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

// NewLeaves returns the calendar CRUD leaf commands, one per file: list.go,
// get.go, create.go, update.go, delete.go.
func NewLeaves(cfg *app.Config, newSvc serviceFunc) []*cobra.Command {
	return []*cobra.Command{
		newListCmd(cfg, newSvc),
		newGetCmd(cfg, newSvc),
		newCreateCmd(cfg, newSvc),
		newUpdateCmd(cfg, newSvc),
		newDeleteCmd(cfg, newSvc),
	}
}
