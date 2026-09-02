package event

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	calendar "google.golang.org/api/calendar/v3"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/google/calendar/dates"
	"github.com/oskarhane/everything-cli/internal/providers/google/calendar/service"
)

// updateFlags are the update write flags; at least one must be set.
var updateFlags = []string{"summary", "start", "end", "location", "description", "add-attendee", "remove-attendee"}

// newUpdateCmd returns `calendar event update`. Patching an instance id
// modifies only that occurrence (the server records an exception); patching
// a master id modifies the whole series. --this-only (default) patches the
// id as given; passing --this-only=false with an instance id patches the
// derived master id instead, i.e. the whole series.
func newUpdateCmd(cfg *app.Config, newSvc service.Dialer[service.EventService]) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <event-id>",
		Short: "Update an event (one occurrence or the series)",
		Example: `# Rename an event
everything-cli google calendar event update abc123 --summary "Design review"

# Move one occurrence of a recurring series (creates an exception)
everything-cli google calendar event update kq3abc123_20260929T030000Z --start 2026-09-29T15:00:00Z --end 2026-09-29T16:00:00Z

# Rename the whole series from any of its occurrences
everything-cli google calendar event update kq3abc123_20260929T030000Z --this-only=false --summary "Standup moved"

# Add a guest to the series master
everything-cli google calendar event update kq3abc123 --add-attendee colleague@example.com`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f := cmd.Flags()
			if !anyUpdateFlagChanged(f) {
				return fmt.Errorf("nothing to update: pass at least one of --summary, --start, --end, --location, --description, --add-attendee, --remove-attendee")
			}
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			calendarID := flagString(f, "calendar")
			// Get first: attendee edits must echo the full array (patch
			// overwrites arrays wholesale), and date-only times are only
			// valid for an all-day event.
			ev, err := svc.GetEvent(cmd.Context(), calendarID, args[0])
			if err != nil {
				return err
			}
			patch, err := buildPatch(f, ev, time.Now())
			if err != nil {
				return err
			}
			target := args[0]
			thisOnly, _ := f.GetBool("this-only")
			if !thisOnly {
				target = masterID(ev, args[0])
			}
			patched, err := svc.PatchEvent(cmd.Context(), calendarID, target, patch, "all")
			if err != nil {
				return err
			}
			printEventView(cmd, cfg, patched)
			return nil
		},
	}
	f := cmd.Flags()
	f.String("summary", "", "New event title")
	f.String("start", "", "New start: RFC3339, or YYYY-MM-DD for an all-day event")
	f.String("end", "", "New end: RFC3339, or YYYY-MM-DD for an all-day event")
	f.String("location", "", "New location")
	f.String("description", "", "New description")
	f.StringArray("add-attendee", nil, "Attendee email to add; repeatable")
	f.StringArray("remove-attendee", nil, "Attendee email to remove; repeatable")
	f.String("calendar", "primary", "Calendar id")
	f.Bool("this-only", true, "Patch the given id only; false with an instance id patches its master (the whole series)")
	return cmd
}

// anyUpdateFlagChanged reports whether any update write flag was set.
func anyUpdateFlagChanged(f *pflag.FlagSet) bool {
	for _, name := range updateFlags {
		if f.Changed(name) {
			return true
		}
	}
	return false
}

// buildPatch assembles the partial patch body: only the changed fields. The
// attendees array, when touched, is the full list with the adds/removes
// applied.
func buildPatch(f *pflag.FlagSet, ev *calendar.Event, now time.Time) (*calendar.Event, error) {
	patch := &calendar.Event{}
	if f.Changed("summary") {
		patch.Summary = flagString(f, "summary")
	}
	if f.Changed("location") {
		patch.Location = flagString(f, "location")
	}
	if f.Changed("description") {
		patch.Description = flagString(f, "description")
	}
	allDay := ev.Start != nil && ev.Start.Date != ""
	for _, name := range []string{"start", "end"} {
		if !f.Changed(name) {
			continue
		}
		et, err := dates.ParseTimestamp(flagString(f, name), now)
		if err != nil {
			return nil, fmt.Errorf("--%s: %w", name, err)
		}
		if et.Date != "" && !allDay {
			return nil, fmt.Errorf("--%s %q is a date but the event is not all-day: pass a full RFC3339 timestamp", name, flagString(f, name))
		}
		timeZone := ""
		if ev.Start != nil {
			timeZone = ev.Start.TimeZone
		}
		if name == "start" {
			patch.Start = toEventDateTime(et, timeZone)
		} else {
			patch.End = toEventDateTime(et, timeZone)
		}
	}
	if f.Changed("add-attendee") || f.Changed("remove-attendee") {
		remove := make(map[string]bool)
		for _, email := range flagStringArray(f, "remove-attendee") {
			remove[email] = true
		}
		attendees := make([]*calendar.EventAttendee, 0, len(ev.Attendees))
		for _, a := range ev.Attendees {
			if !remove[a.Email] {
				attendees = append(attendees, a)
			}
		}
		for _, email := range flagStringArray(f, "add-attendee") {
			attendees = append(attendees, &calendar.EventAttendee{Email: email})
		}
		patch.Attendees = attendees
	}
	return patch, nil
}
