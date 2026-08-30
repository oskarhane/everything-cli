package calendarlist

import (
	"github.com/spf13/pflag"

	calendar "google.golang.org/api/calendar/v3"
)

// addCalendarFlags registers the optional write flags shared by create and
// update. create additionally takes the summary positionally; update adds
// --summary, so only "summary" is read where the flag exists.
func addCalendarFlags(f *pflag.FlagSet) {
	f.String("timezone", "", "IANA time zone, e.g. Europe/Stockholm")
	f.String("description", "", "Calendar description")
	f.String("color-id", "", "Color id from the calendar colors endpoint, e.g. tomato")
}

// applyCalendarFlags writes the --description and --timezone flags set on f
// into cal, leaving unset fields alone so update sends a partial body.
// --color-id is handled separately: colorId lives on the calendar list
// entry, not the Calendar resource, so it is patched through PatchCalendarList.
func applyCalendarFlags(cal *calendar.Calendar, f *pflag.FlagSet) {
	if f.Changed("description") {
		cal.Description, _ = f.GetString("description")
	}
	if f.Changed("timezone") {
		cal.TimeZone, _ = f.GetString("timezone")
	}
}

// colorID returns the --color-id value and whether the flag was set.
func colorID(f *pflag.FlagSet) (string, bool) {
	if !f.Changed("color-id") {
		return "", false
	}
	v, _ := f.GetString("color-id")
	return v, true
}

// anyCalendarFlagChanged reports whether any of the update write flags was
// set on f, so update can refuse an empty modification.
func anyCalendarFlagChanged(f *pflag.FlagSet) bool {
	for _, name := range []string{"summary", "description", "timezone", "color-id"} {
		if f.Changed(name) {
			return true
		}
	}
	return false
}

// anyCalendarResourceFlagChanged reports whether any of the flags carried by
// the Calendar resource was set, so update only calls Calendars.Patch when
// there is something to patch there.
func anyCalendarResourceFlagChanged(f *pflag.FlagSet) bool {
	for _, name := range []string{"summary", "description", "timezone"} {
		if f.Changed(name) {
			return true
		}
	}
	return false
}
