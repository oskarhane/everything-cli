package calendarlist

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/providers/google/calendar/service"
)

// newDeleteCmd returns `calendar delete`: remove a calendar. Deleting is
// destructive and removes every event on the calendar, so it refuses to run
// without --force.
func newDeleteCmd(_ *app.Config, newSvc service.Dialer[service.CalendarService]) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "delete <calendar-id>",
		Short: "Delete a calendar (destructive)",
		Example: `# See the refusal without --force
google-cli calendar delete abc123.group.calendar.google.com

# Actually delete the calendar
google-cli calendar delete abc123.group.calendar.google.com --force`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !force {
				return fmt.Errorf("refusing to delete calendar %q without --force (this deletes the calendar and every event on it)", args[0])
			}
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			return svc.DeleteCalendar(cmd.Context(), args[0])
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Delete the calendar instead of refusing")
	return cmd
}
