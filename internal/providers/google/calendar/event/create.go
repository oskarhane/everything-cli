package event

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	calendar "google.golang.org/api/calendar/v3"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/providers/google/calendar/dates"
	"github.com/oskarhane/google-cli/internal/providers/google/calendar/service"
)

// newCreateCmd returns `calendar event create`. --recurrence takes raw
// RRULE:/RDATE:/EXDATE: values and forwards them verbatim into the event's
// recurrence lines; --all-day switches --start/--end to YYYY-MM-DD dates.
func newCreateCmd(cfg *app.Config, newSvc service.Dialer[service.EventService]) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an event",
		Example: `# Create a one-off meeting
google-cli calendar event create --summary "Design review" --start 2026-09-03T14:00:00Z --end 2026-09-03T15:00:00Z

# Create a weekly recurring series with a guest, as JSON
google-cli calendar event create --summary "Standup" --start 2026-09-01T09:00:00+02:00 --end 2026-09-01T09:30:00+02:00 --attendee colleague@example.com --recurrence 'RRULE:FREQ=WEEKLY;COUNT=10' --format json

# Create an all-day event spanning three days
google-cli calendar event create --summary "Conference" --start 2026-10-01 --end 2026-10-03 --all-day`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			f := cmd.Flags()
			summary, _ := f.GetString("summary")
			if summary == "" {
				return fmt.Errorf("--summary is required: the event title")
			}
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			ev, err := buildEvent(f, time.Now())
			if err != nil {
				return err
			}
			calendarID, _ := f.GetString("calendar")
			sendUpdates := "none"
			if len(ev.Attendees) > 0 {
				sendUpdates = "all"
			}
			created, err := svc.InsertEvent(cmd.Context(), calendarID, ev, sendUpdates)
			if err != nil {
				return err
			}
			printEventView(cmd, cfg, created)
			return nil
		},
	}
	f := cmd.Flags()
	f.String("summary", "", "Event title (required)")
	f.String("start", "", "Start: RFC3339, or YYYY-MM-DD with --all-day (required)")
	f.String("end", "", "End: RFC3339, or YYYY-MM-DD with --all-day (required)")
	f.String("calendar", "primary", "Calendar id")
	f.Bool("all-day", false, "Treat --start/--end as YYYY-MM-DD dates")
	f.String("timezone", "", "IANA time zone, e.g. Europe/Stockholm; required for recurring series")
	f.String("location", "", "Location")
	f.String("description", "", "Description")
	f.StringArray("attendee", nil, "Attendee email; repeatable")
	f.Int64("reminder-minutes", 0, "Popup reminder this many minutes before (0 = calendar default)")
	f.String("color-id", "", "Color id from the event colors endpoint")
	f.StringArray("recurrence", nil, "Raw RRULE:/RDATE:/EXDATE: value, e.g. 'RRULE:FREQ=WEEKLY;COUNT=10'; repeatable")
	return cmd
}

// buildEvent assembles the insert body from the create flags.
func buildEvent(f *pflag.FlagSet, now time.Time) (*calendar.Event, error) {
	startRaw, _ := f.GetString("start")
	endRaw, _ := f.GetString("end")
	if startRaw == "" || endRaw == "" {
		return nil, fmt.Errorf("--start and --end are required")
	}
	allDay, _ := f.GetBool("all-day")
	timeZone, _ := f.GetString("timezone")

	start, err := dates.ParseTimestamp(startRaw, now)
	if err != nil {
		return nil, fmt.Errorf("--start: %w", err)
	}
	end, err := dates.ParseTimestamp(endRaw, now)
	if err != nil {
		return nil, fmt.Errorf("--end: %w", err)
	}
	if start.Date != "" && !allDay {
		return nil, fmt.Errorf("--start %q is a date: pass --all-day or a full RFC3339 timestamp", startRaw)
	}
	if end.Date != "" && !allDay {
		return nil, fmt.Errorf("--end %q is a date: pass --all-day or a full RFC3339 timestamp", endRaw)
	}

	ev := &calendar.Event{
		Summary:     flagString(f, "summary"),
		Start:       toEventDateTime(start, timeZone),
		End:         toEventDateTime(end, timeZone),
		Location:    flagString(f, "location"),
		Description: flagString(f, "description"),
		Recurrence:  flagStringArray(f, "recurrence"),
	}
	if color := flagString(f, "color-id"); color != "" {
		ev.ColorId = color
	}
	for _, email := range flagStringArray(f, "attendee") {
		ev.Attendees = append(ev.Attendees, &calendar.EventAttendee{Email: email})
	}
	if minutes, _ := f.GetInt64("reminder-minutes"); minutes > 0 {
		// An explicit override must also disable the calendar's default
		// reminders; UseDefault is omitempty, so force-send the false.
		ev.Reminders = &calendar.EventReminders{
			UseDefault:      false,
			Overrides:       []*calendar.EventReminder{{Method: "popup", Minutes: minutes}},
			ForceSendFields: []string{"UseDefault"},
		}
	}
	return ev, nil
}

// flagString and flagStringArray read flags whose errors cannot happen (the
// flag exists and matches the type).
func flagString(f *pflag.FlagSet, name string) string {
	v, _ := f.GetString(name)
	return v
}

func flagStringArray(f *pflag.FlagSet, name string) []string {
	v, _ := f.GetStringArray(name)
	return v
}
