package calendarlist

import (
	"github.com/spf13/cobra"

	calendar "google.golang.org/api/calendar/v3"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/providers/google/calendar/service"
)

// newCreateCmd returns `calendar create`: a new secondary calendar. When
// --color-id is set the created calendar's list entry is patched, because
// colorId lives on the calendar list entry, not the Calendar resource.
func newCreateCmd(cfg *app.Config, newSvc service.Dialer[service.CalendarService]) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <summary>",
		Short: "Create a calendar",
		Example: `# Create a calendar
google-cli calendar create "Team PTO"

# Create a calendar with a timezone, description, and color
google-cli calendar create "Team PTO" --timezone Europe/Stockholm --description "Shared time off" --color-id tomato`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cal := &calendar.Calendar{Summary: args[0]}
			applyCalendarFlags(cal, cmd.Flags())
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			created, err := svc.InsertCalendar(cmd.Context(), cal)
			if err != nil {
				return err
			}
			var entry *calendar.CalendarListEntry
			if color, ok := colorID(cmd.Flags()); ok {
				entry, err = svc.PatchCalendarList(cmd.Context(), created.Id, &calendar.CalendarListEntry{ColorId: color})
				if err != nil {
					return err
				}
			}
			printCalendar(cmd, cfg, calendarRow(created, entry, ""))
			return nil
		},
	}
	addCalendarFlags(cmd.Flags())
	return cmd
}
