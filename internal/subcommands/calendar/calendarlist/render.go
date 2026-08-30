package calendarlist

import (
	"io"

	"github.com/spf13/cobra"

	calendar "google.golang.org/api/calendar/v3"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/output"
)

// calendarFields is the single-calendar field order for table output; the
// same names are the snake_case JSON and TOON keys. go-pretty's StyleLight
// upper-cases the headers when rendering.
var calendarFields = []string{"id", "summary", "description", "timezone", "color_id"}

// calendarListFields is the calendar-list row field order.
var calendarListFields = []string{"id", "summary", "timezone", "primary"}

// calendarListRow maps one calendar list entry to its output row.
func calendarListRow(e *calendar.CalendarListEntry) map[string]any {
	return map[string]any{
		"id":       e.Id,
		"summary":  e.Summary,
		"timezone": e.TimeZone,
		"primary":  e.Primary,
	}
}

// calendarRow merges a Calendar resource with its calendar list entry.
// colorId lives only on the list entry, so reads and writes pair the two: a
// nil entry leaves color_id empty. The Calendar resource, when present, wins
// on the fields it carries; fallbackID covers responses without an id.
func calendarRow(cal *calendar.Calendar, entry *calendar.CalendarListEntry, fallbackID string) map[string]any {
	row := map[string]any{
		"id":          fallbackID,
		"summary":     "",
		"description": "",
		"timezone":    "",
		"color_id":    "",
	}
	if entry != nil {
		if entry.Id != "" {
			row["id"] = entry.Id
		}
		row["summary"] = entry.Summary
		row["description"] = entry.Description
		row["timezone"] = entry.TimeZone
		row["color_id"] = entry.ColorId
	}
	if cal != nil {
		if cal.Id != "" {
			row["id"] = cal.Id
		}
		row["summary"] = cal.Summary
		row["description"] = cal.Description
		row["timezone"] = cal.TimeZone
	}
	return row
}

// printCalendarList renders the calendar list: a JSON/TOON array, or a table
// with one row per calendar, in the resolved output format.
func printCalendarList(cmd *cobra.Command, cfg *app.Config, entries []*calendar.CalendarListEntry) {
	rows := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		rows = append(rows, calendarListRow(e))
	}
	render(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format), calendarListFields, rows, rows)
}

// printCalendar renders a single calendar row: an object in JSON/TOON, a
// one-row table.
func printCalendar(cmd *cobra.Command, cfg *app.Config, row map[string]any) {
	render(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format), calendarFields, row, []map[string]any{row})
}

// render writes v (JSON/TOON) or rows (table) in the given format.
func render(w io.Writer, format output.Format, fields []string, v any, rows []map[string]any) {
	switch format {
	case output.FormatTable:
		output.PrintTable(w, fields, rows)
	case output.FormatToon:
		output.PrintToon(w, v)
	default:
		output.PrintJSON(w, v)
	}
}
