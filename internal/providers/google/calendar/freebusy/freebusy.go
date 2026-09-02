// Package freebusy builds the `calendar freebusy` command: the busy time
// intervals of one or all of an account's calendars.
package freebusy

import (
	"sort"
	"time"

	"github.com/spf13/cobra"

	calendar "google.golang.org/api/calendar/v3"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
	"github.com/oskarhane/everything-cli/internal/providers/google/calendar/dates"
	"github.com/oskarhane/everything-cli/internal/providers/google/calendar/service"
)

// nowFunc is the clock seam: tests pin it so the default window
// (now .. now+1d) is deterministic.
var nowFunc = time.Now

// busyFields is the busy-period row field order for table output; the same
// names are the snake_case JSON and TOON keys. go-pretty's StyleLight
// upper-cases the headers when rendering.
var busyFields = []string{"calendar_id", "start", "end"}

// NewCmd returns `calendar freebusy`. Calendars default to every entry on
// the account's calendar list; --calendar picks explicit ones (repeatable).
func NewCmd(cfg *app.Config, newSvc service.Dialer[service.FreeBusyService]) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "freebusy",
		Short: "List busy time intervals for one or all calendars",
		Example: `# Free/busy for every listed calendar over the next day, as JSON
everything-cli google calendar freebusy --format json

# Free/busy for two named calendars during a fixed window
everything-cli google calendar freebusy --from 2026-09-01T09:00:00Z --to 2026-09-01T17:00:00Z --calendar work@example.com --calendar personal@example.com --format table

# This afternoon's busy slots on one calendar, relative window
everything-cli google calendar freebusy --from now --to +8h --calendar primary --format json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			periods, err := queryBusyPeriods(cmd, svc)
			if err != nil {
				return err
			}
			printBusyPeriods(cmd, cfg, periods)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringSlice("calendar", nil, "Calendar id to query; repeatable (default: every listed calendar)")
	f.String("from", "now", "Window start: RFC3339, date, or relative (now, -1d, +8h)")
	f.String("to", "+1d", "Window end: RFC3339, date, or relative")
	return cmd
}

// queryBusyPeriods resolves the calendar ids (--calendar values, else every
// entry on the calendar list), runs the freebusy query, and expands the
// response into one row per busy period.
func queryBusyPeriods(cmd *cobra.Command, svc service.FreeBusyService) ([]map[string]any, error) {
	f := cmd.Flags()
	now := nowFunc()
	fromRaw, _ := f.GetString("from")
	toRaw, _ := f.GetString("to")
	from, err := dates.ParseWindowTime(fromRaw, now)
	if err != nil {
		return nil, err
	}
	to, err := dates.ParseWindowTime(toRaw, now)
	if err != nil {
		return nil, err
	}
	calendarIDs, _ := f.GetStringSlice("calendar")
	if len(calendarIDs) == 0 {
		entries, err := svc.ListCalendarList(cmd.Context())
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			calendarIDs = append(calendarIDs, entry.Id)
		}
	}
	resp, err := svc.QueryFreeBusy(cmd.Context(), service.QueryFreeBusyParams{
		TimeMin:     from,
		TimeMax:     to,
		CalendarIDs: calendarIDs,
	})
	if err != nil {
		return nil, err
	}
	return expandBusyPeriods(resp), nil
}

// expandBusyPeriods flattens the response's per-calendar busy lists into one
// row per period, ordered by calendar id then start so output is stable.
func expandBusyPeriods(resp *calendar.FreeBusyResponse) []map[string]any {
	rows := make([]map[string]any, 0)
	ids := make([]string, 0, len(resp.Calendars))
	for id := range resp.Calendars {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		for _, period := range resp.Calendars[id].Busy {
			rows = append(rows, map[string]any{
				"calendar_id": id,
				"start":       period.Start,
				"end":         period.End,
			})
		}
	}
	return rows
}

// printBusyPeriods renders zero or more busy periods: a JSON/TOON array, or
// a table with one row per period, in the resolved output format. A single
// busy period collapses to one JSON/TOON object, the package-wide one-row
// convention.
func printBusyPeriods(cmd *cobra.Command, cfg *app.Config, periods []map[string]any) {
	output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format), busyFields, periods, periods)
}
