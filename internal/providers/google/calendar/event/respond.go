package event

import (
	"fmt"

	"github.com/spf13/cobra"

	calendar "google.golang.org/api/calendar/v3"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/subcommands/calendar/service"
)

// respondVerbs maps each response leaf to its attendees[].responseStatus
// value; the leaf names double as the verbs.
var respondVerbs = map[string]string{
	"accept":    "accepted",
	"decline":   "declined",
	"tentative": "tentative",
}

// newRespondCmd returns one of `calendar event accept|decline|tentative`.
// The recipe: get the event, flip the attendee entry marked self, patch the
// SAME id with the full attendees array (patch overwrites arrays wholesale).
// An instance id responds to that one occurrence only (creating an
// exception); a master id, or --all with an instance id, responds for the
// whole series.
func newRespondCmd(cfg *app.Config, newSvc service.Dialer[service.EventService], verb string) *cobra.Command {
	status := respondVerbs[verb]
	cmd := &cobra.Command{
		Use:   verb + " <event-id>",
		Short: respondShort(verb) + " an event invitation",
		Example: fmt.Sprintf(`# %s a single event
google-cli calendar event %s abc123

# Respond to only one occurrence of a recurring series (instance ids end in _<UTC time>)
google-cli calendar event %s kq3abc123_20260929T030000Z

# Respond for the entire recurring series from any of its occurrences
google-cli calendar event %s kq3abc123_20260929T030000Z --all`, respondShort(verb), verb, verb, verb),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f := cmd.Flags()
			thisOnly, _ := f.GetBool("this-only")
			all, _ := f.GetBool("all")
			if all {
				thisOnly = false
			}
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			calendarID := flagString(f, "calendar")
			ev, err := svc.GetEvent(cmd.Context(), calendarID, args[0])
			if err != nil {
				return err
			}
			target := args[0]
			if !thisOnly {
				// --all: respond on the master id so the series-level
				// attendees carry the new response. Refetch so the patch
				// echoes the master's attendee list, not the instance's.
				if master := masterID(ev, args[0]); master != target {
					target = master
					ev, err = svc.GetEvent(cmd.Context(), calendarID, target)
					if err != nil {
						return err
					}
				}
			}
			self := -1
			for i, a := range ev.Attendees {
				if a.Self {
					self = i
					break
				}
			}
			if self < 0 {
				return fmt.Errorf("you are not an attendee of this event")
			}
			ev.Attendees[self].ResponseStatus = status
			patched, err := svc.PatchEvent(cmd.Context(), calendarID, target, &calendar.Event{Attendees: ev.Attendees}, "none")
			if err != nil {
				return err
			}
			printEventView(cmd, cfg, patched)
			return nil
		},
	}
	f := cmd.Flags()
	f.String("calendar", "primary", "Calendar id")
	f.Bool("this-only", true, "Respond for the given id only (an instance id means that one occurrence)")
	f.Bool("all", false, "Respond for the whole series (overrides --this-only)")
	return cmd
}

// respondShort returns the leaf's verb summary.
func respondShort(verb string) string {
	switch verb {
	case "accept":
		return "Accept"
	case "decline":
		return "Decline"
	default:
		return "Tentatively accept"
	}
}
