package calendarlist

import (
	"fmt"

	"github.com/spf13/cobra"

	calendar "google.golang.org/api/calendar/v3"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/providers/google/calendar/service"
)

// newUpdateCmd returns `calendar update`: modify an existing calendar. Only
// the flags that were set are sent, so a partial update leaves the other
// calendar fields untouched. --summary, --description, and --timezone patch
// the Calendar resource; --color-id patches its calendar list entry.
func newUpdateCmd(cfg *app.Config, newSvc service.Dialer[service.CalendarService]) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <calendar-id>",
		Short: "Update a calendar",
		Example: `# Rename a calendar
google-cli calendar update abc123.group.calendar.google.com --summary "Team PTO 2026"

# Change the timezone and color
google-cli calendar update abc123.group.calendar.google.com --timezone Europe/Stockholm --color-id tomato`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f := cmd.Flags()
			if !anyCalendarFlagChanged(f) {
				return fmt.Errorf("nothing to update: pass at least one of --summary, --description, --timezone, --color-id")
			}
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			var cal *calendar.Calendar
			if anyCalendarResourceFlagChanged(f) {
				patch := &calendar.Calendar{}
				if f.Changed("summary") {
					patch.Summary, _ = f.GetString("summary")
				}
				applyCalendarFlags(patch, f)
				cal, err = svc.PatchCalendar(cmd.Context(), args[0], patch)
				if err != nil {
					return err
				}
			}
			var entry *calendar.CalendarListEntry
			if color, ok := colorID(f); ok {
				entry, err = svc.PatchCalendarList(cmd.Context(), args[0], &calendar.CalendarListEntry{ColorId: color})
				if err != nil {
					return err
				}
			}
			printCalendar(cmd, cfg, calendarRow(cal, entry, args[0]))
			return nil
		},
	}
	cmd.Flags().String("summary", "", "New calendar title")
	addCalendarFlags(cmd.Flags())
	return cmd
}
