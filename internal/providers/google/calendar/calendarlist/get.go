package calendarlist

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/providers/google/calendar/service"
)

// newGetCmd returns `calendar get`: one calendar by id. The Calendar
// resource carries id, summary, description, and timezone; its color_id
// lives on the calendar list entry, so get also fetches that entry.
func newGetCmd(cfg *app.Config, newSvc service.Dialer[service.CalendarService]) *cobra.Command {
	return &cobra.Command{
		Use:   "get <calendar-id>",
		Short: "Show a calendar by id",
		Example: `# Show the primary calendar as JSON
google-cli calendar get primary --format json

# Show a secondary calendar as a table
google-cli calendar get abc123.group.calendar.google.com --format table`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			cal, err := svc.GetCalendar(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			entry, err := svc.GetCalendarList(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			printCalendar(cmd, cfg, calendarRow(cal, entry, args[0]))
			return nil
		},
	}
}
