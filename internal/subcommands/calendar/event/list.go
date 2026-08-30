package event

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	calendar "google.golang.org/api/calendar/v3"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/subcommands/calendar/service"
)

// recurringModes are the accepted --recurring values.
var recurringModes = map[string]bool{"instances": true, "masters": true, "all": true}

// newListCmd returns `calendar event list`. --recurring picks how recurring
// events appear: instances (default) expands series into occurrences via
// singleEvents=true; masters returns the raw list (masters, single events,
// and existing exceptions); all merges both calls. Expansion is always
// bounded by timeMax so unbounded RRULEs cannot page forever.
func newListCmd(cfg *app.Config, newSvc service.Dialer[service.EventService]) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List events, expanding or hiding recurring series",
		Example: `# This week's events as JSON, recurring series expanded into occurrences
google-cli calendar event list --from 2026-09-01T00:00:00Z --to 2026-09-08T00:00:00Z --format json

# Show the underlying recurring masters and one-off events instead
google-cli calendar event list --recurring masters --format table

# Search events by keyword on another calendar
google-cli calendar event list --calendar work@example.com --query "design review" --max 10`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			f := cmd.Flags()
			mode, _ := f.GetString("recurring")
			if !recurringModes[mode] {
				return fmt.Errorf("invalid --recurring %q: expected instances, masters, or all", mode)
			}
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			events, err := listEvents(cmd, svc, mode)
			if err != nil {
				return err
			}
			printEventList(cmd, cfg, events)
			return nil
		},
	}
	f := cmd.Flags()
	f.String("calendar", "primary", "Calendar id")
	f.String("from", "now", "Window start: RFC3339, date, or relative (now, -1d, +7d)")
	f.String("to", "+7d", "Window end: RFC3339, date, or relative; expansion is always bounded")
	f.String("query", "", "Free-text search term")
	f.Int64("max", 0, "Max results (0 = API default page size)")
	f.String("recurring", "instances", "Show instances (expanded occurrences), masters, or all")
	return cmd
}

// listEvents fetches events for one --recurring mode. The instances call
// expands with singleEvents=true and orderBy startTime; the masters call is
// the default list. The window defaults (now .. +7d) bound both calls.
func listEvents(cmd *cobra.Command, svc service.EventService, mode string) ([]*calendar.Event, error) {
	f := cmd.Flags()
	calendarID, _ := f.GetString("calendar")
	query, _ := f.GetString("query")
	max, _ := f.GetInt64("max")
	fromRaw, _ := f.GetString("from")
	toRaw, _ := f.GetString("to")
	now := time.Now()

	from, err := parseWindowTime(fromRaw, now)
	if err != nil {
		return nil, err
	}
	to, err := parseWindowTime(toRaw, now)
	if err != nil {
		return nil, err
	}
	base := service.ListEventsParams{
		CalendarID: calendarID,
		TimeMin:    from,
		TimeMax:    to,
		Query:      query,
		MaxResults: max,
	}
	if mode == "masters" {
		return svc.ListEvents(cmd.Context(), base)
	}
	instances, err := svc.ListEvents(cmd.Context(), service.ListEventsParams{
		CalendarID:   base.CalendarID,
		SingleEvents: true,
		TimeMin:      base.TimeMin,
		TimeMax:      base.TimeMax,
		Query:        base.Query,
		MaxResults:   base.MaxResults,
		OrderBy:      "startTime",
	})
	if err != nil {
		return nil, err
	}
	if mode == "instances" {
		return instances, nil
	}
	// all: merge the expanded occurrences with the masters/exceptions list,
	// deduping by id — exceptions appear in both responses.
	events := instances
	seen := make(map[string]bool, len(events))
	for _, ev := range events {
		seen[ev.Id] = true
	}
	masters, err := svc.ListEvents(cmd.Context(), base)
	if err != nil {
		return nil, err
	}
	for _, ev := range masters {
		if !seen[ev.Id] {
			events = append(events, ev)
		}
	}
	return events, nil
}
