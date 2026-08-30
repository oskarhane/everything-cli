// Package freebusy builds the `calendar freebusy` command: the busy time
// intervals of one or all of an account's calendars.
package freebusy

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	calendar "google.golang.org/api/calendar/v3"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/output"
	"github.com/oskarhane/google-cli/internal/subcommands/calendar/service"
)

// serviceFunc builds the freebusy service a leaf's RunE uses. The calendar
// parent injects the real dialer; tests inject fakes so the leaf never
// touches the network.
type serviceFunc func(context.Context) (service.FreeBusyService, error)

// nowFunc is the clock seam: tests pin it so the default window
// (now .. now+1d) is deterministic.
var nowFunc = time.Now

// busyFields is the busy-period row field order for table output; the same
// names are the snake_case JSON and TOON keys. go-pretty's StyleLight
// upper-cases the headers when rendering.
var busyFields = []string{"calendar_id", "start", "end"}

// NewCmd returns `calendar freebusy`. Calendars default to every entry on
// the account's calendar list; --calendar picks explicit ones (repeatable).
func NewCmd(cfg *app.Config, newSvc serviceFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "freebusy",
		Short: "List busy time intervals for one or all calendars",
		Example: `# Free/busy for every listed calendar over the next day, as JSON
google-cli calendar freebusy --format json

# Free/busy for two named calendars during a fixed window
google-cli calendar freebusy --from 2026-09-01T09:00:00Z --to 2026-09-01T17:00:00Z --calendar work@example.com --calendar personal@example.com --format table

# This afternoon's busy slots on one calendar, relative window
google-cli calendar freebusy --from now --to +8h --calendar primary --format json`,
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
	from, err := parseWindowTime(fromRaw, now)
	if err != nil {
		return nil, err
	}
	to, err := parseWindowTime(toRaw, now)
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
// a table with one row per period, in the resolved output format.
func printBusyPeriods(cmd *cobra.Command, cfg *app.Config, periods []map[string]any) {
	switch output.ResolveOutput(cfg.Format) {
	case output.FormatTable:
		output.PrintTable(cmd.OutOrStdout(), busyFields, periods)
	case output.FormatToon:
		output.PrintToon(cmd.OutOrStdout(), periods)
	default:
		output.PrintJSON(cmd.OutOrStdout(), periods)
	}
}

// Window parsing mirrors event/dates.go (that parser is unexported): the
// freebusy window bounds accept RFC3339, a date, or a relative offset.

const dateOnlyLayout = "2006-01-02"

// relativeRe matches relative offsets like -1d, +1d, -30m, +8h. A bare count
// (1d) counts forward, like +1d. "now" is handled before the regexp runs.
var relativeRe = regexp.MustCompile(`^([-+]?\d+)([dhm])$`)

// parseWindowTime parses one --from/--to value into the RFC3339 the
// freebusy.query request requires. Accepted forms: RFC3339 with an offset,
// a date (local midnight), or a relative offset (now, -1d, +1d, -30m, +8h).
// now anchors relative values; callers inject it so tests are deterministic.
func parseWindowTime(value string, now time.Time) (string, error) {
	if value == "now" {
		return now.Format(time.RFC3339), nil
	}
	if m := relativeRe.FindStringSubmatch(value); m != nil {
		n, err := strconv.Atoi(m[1])
		if err != nil {
			return "", fmt.Errorf("invalid relative time %q: %w", value, err)
		}
		return now.Add(time.Duration(n) * relativeUnit(m[2])).Format(time.RFC3339), nil
	}
	if _, err := time.Parse(time.RFC3339, value); err == nil {
		return value, nil
	}
	if t, err := time.ParseInLocation(dateOnlyLayout, value, now.Location()); err == nil {
		return t.Format(time.RFC3339), nil
	}
	return "", fmt.Errorf("invalid timestamp %q: expected RFC3339 (2026-09-03T14:00:00Z), a date (2026-09-03), or a relative offset (now, -1d, +1d, -30m, +2h)", value)
}

// relativeUnit maps a relative-offset unit letter to its duration.
func relativeUnit(unit string) time.Duration {
	switch unit {
	case "d":
		return 24 * time.Hour
	case "h":
		return time.Hour
	default: // "m"
		return time.Minute
	}
}
