package event

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	calendar "google.golang.org/api/calendar/v3"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/providers/google/calendar/service"
)

// nowFunc is the clock seam: tests pin it so the default window
// (now .. now+7d) is deterministic.
var nowFunc = time.Now

// recurringModes are the accepted --recurring values.
var recurringModes = map[string]bool{"instances": true, "masters": true, "all": true}

// defaultListMax is the default --max for both listing leaves: the Calendar
// API's own default page size, so no cap-fewer-request behavior changes for
// the common case while a large calendar can no longer page forever.
const defaultListMax = 250

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

# Change-detection pull: only events modified since yesterday
google-cli calendar event list --updated-since -1d --format json

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
	f.String("from", "now", "Window start: RFC3339 (offset optional), date, or relative (now, -1d, +7d)")
	f.String("to", "+7d", "Window end: RFC3339 (offset optional), date, or relative; expansion is always bounded")
	f.Bool("show-deleted", true, "Include cancelled events (status \"cancelled\")")
	f.String("updated-since", "", "Only events modified since: RFC3339 (offset optional), date, or relative (now, -1d); empty = no filter")
	f.String("query", "", "Free-text search term")
	f.Int64("max", defaultListMax, "Total max events across all pages (0 = no cap)")
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
	showDeleted, _ := f.GetBool("show-deleted")
	updatedSinceRaw, _ := f.GetString("updated-since")
	now := nowFunc()

	from, err := parseWindowTime(fromRaw, now)
	if err != nil {
		return nil, err
	}
	to, err := parseWindowTime(toRaw, now)
	if err != nil {
		return nil, err
	}
	updatedMin, err := parseWindowTime(updatedSinceRaw, now)
	if err != nil {
		return nil, err
	}
	base := service.ListEventsParams{
		CalendarID:  calendarID,
		TimeMin:     from,
		TimeMax:     to,
		Query:       query,
		MaxResults:  max,
		ShowDeleted: showDeleted,
		UpdatedMin:  updatedMin,
	}
	if mode == "masters" {
		return svc.ListEvents(cmd.Context(), base)
	}
	// Instances expand the base window with singleEvents + startTime order.
	p := base
	p.SingleEvents = true
	p.OrderBy = "startTime"
	instances, err := svc.ListEvents(cmd.Context(), p)
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
