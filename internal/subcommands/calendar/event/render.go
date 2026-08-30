package event

import (
	"io"
	"strings"

	"github.com/spf13/cobra"

	calendar "google.golang.org/api/calendar/v3"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/output"
)

// eventListFields is the event list row field order for table output; the
// same names are the snake_case JSON and TOON keys. go-pretty's StyleLight
// upper-cases the headers when rendering.
var eventListFields = []string{"id", "summary", "start", "end", "recurring", "recurring_event_id"}

// eventViewFields is the single-event view field order.
var eventViewFields = []string{"id", "summary", "start", "end", "location", "description", "attendees", "recurring", "recurring_event_id", "recurrence"}

// dateTimeString renders an EventDateTime as one string: the date for
// all-day events, the RFC3339 datetime otherwise.
func dateTimeString(t *calendar.EventDateTime) string {
	if t == nil {
		return ""
	}
	if t.Date != "" {
		return t.Date
	}
	return t.DateTime
}

// eventListRow maps one event to its list row. recurring is true for masters
// (recurrence set) and instances (recurringEventId set); recurring_event_id
// is empty for masters and single events.
func eventListRow(ev *calendar.Event) map[string]any {
	return map[string]any{
		"id":                 ev.Id,
		"summary":            ev.Summary,
		"start":              dateTimeString(ev.Start),
		"end":                dateTimeString(ev.End),
		"recurring":          isRecurring(ev),
		"recurring_event_id": ev.RecurringEventId,
	}
}

// attendeeRows maps the attendee list for the view: email plus each guest's
// response status.
func attendeeRows(attendees []*calendar.EventAttendee) []map[string]any {
	rows := make([]map[string]any, 0, len(attendees))
	for _, a := range attendees {
		rows = append(rows, map[string]any{
			"email":           a.Email,
			"response_status": a.ResponseStatus,
		})
	}
	return rows
}

// eventView maps one event to the get/create/update output shape. recurrence
// carries the master's RRULE/RDATE/EXDATE lines; instances and single events
// get an empty list.
func eventView(ev *calendar.Event) map[string]any {
	recurrence := ev.Recurrence
	if recurrence == nil {
		recurrence = []string{}
	}
	return map[string]any{
		"id":                 ev.Id,
		"summary":            ev.Summary,
		"start":              dateTimeString(ev.Start),
		"end":                dateTimeString(ev.End),
		"location":           ev.Location,
		"description":        ev.Description,
		"attendees":          attendeeRows(ev.Attendees),
		"recurring":          isRecurring(ev),
		"recurring_event_id": ev.RecurringEventId,
		"recurrence":         recurrence,
	}
}

// eventViewTableRow flattens the view's list-valued fields into table cells:
// attendees render as "email (status)" pairs, recurrence as the raw lines.
func eventViewTableRow(view map[string]any) map[string]any {
	row := make(map[string]any, len(view))
	for k, v := range view {
		row[k] = v
	}
	parts := make([]string, 0, len(view["attendees"].([]map[string]any)))
	for _, a := range view["attendees"].([]map[string]any) {
		parts = append(parts, a["email"].(string)+" ("+a["response_status"].(string)+")")
	}
	row["attendees"] = strings.Join(parts, ", ")
	lines := make([]string, 0, len(view["recurrence"].([]string)))
	for _, r := range view["recurrence"].([]string) {
		if r != "" {
			lines = append(lines, r)
		}
	}
	row["recurrence"] = strings.Join(lines, "; ")
	return row
}

// printEventList renders zero or more events: a JSON/TOON array, or a table
// with one row per event, in the resolved output format.
func printEventList(cmd *cobra.Command, cfg *app.Config, events []*calendar.Event) {
	rows := make([]map[string]any, 0, len(events))
	for _, ev := range events {
		rows = append(rows, eventListRow(ev))
	}
	render(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format), eventListFields, rows, rows)
}

// printEventView renders a single event: an object in JSON/TOON, a one-row
// table. Table rows flatten the attendee and recurrence lists into cells.
func printEventView(cmd *cobra.Command, cfg *app.Config, ev *calendar.Event) {
	view := eventView(ev)
	render(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format), eventViewFields, view, []map[string]any{eventViewTableRow(view)})
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
