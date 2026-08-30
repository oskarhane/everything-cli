package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	toon "github.com/toon-format/toon-go"

	"github.com/jedib0t/go-pretty/v6/table"
)

// StripControl replaces C0 control characters (runes < 0x20) and DEL (0x7F)
// with "?", except for the whitespace runes "\t", "\n", and "\r" which are
// preserved. This sanitises raw user data before rendering into a terminal
// table cell or a TOON document so that an embedded ANSI escape (e.g.
// "\x1b[31m") cannot inject styling, move the cursor, or make the toon
// encoder error out. The JSON-marshal path already escapes these bytes via
// encoding/json so it does not need this helper.
func StripControl(s string) string {
	if s == "" {
		return s
	}
	// Fast path: scan for any rune that needs replacing before allocating.
	needs := false
	for _, r := range s {
		if (r < 0x20 && r != '\t' && r != '\n' && r != '\r') || r == 0x7F {
			needs = true
			break
		}
	}
	if !needs {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if (r < 0x20 && r != '\t' && r != '\n' && r != '\r') || r == 0x7F {
			b.WriteByte('?')
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// toonMarshal is the seam over toon.Marshal so tests can drive the
// fall-back-to-JSON path without having to construct a value toon rejects.
var toonMarshal = toon.Marshal

// PrintJSON writes v as tab-indented JSON with a trailing newline.
func PrintJSON(w io.Writer, v any) {
	b, err := json.MarshalIndent(v, "", "\t")
	if err != nil {
		panic(err)
	}
	writeLine(w, string(b))
}

// PrintTable writes rows as a go-pretty table (StyleLight) with one column
// per entry in fields. A field of the form "a:b" addresses the nested key
// path a.b in a row. go-pretty's StyleLight upper-cases header cells, so
// callers pass snake_case field names and get upper-case headers.
func PrintTable(w io.Writer, fields []string, rows []map[string]any) {
	t := table.NewWriter()

	header := make(table.Row, 0, len(fields))
	for _, f := range fields {
		header = append(header, f)
	}
	t.AppendHeader(header)

	for _, row := range rows {
		r := make(table.Row, 0, len(fields))
		for _, f := range fields {
			r = append(r, cellValue(row, strings.Split(f, ":")))
		}
		t.AppendRow(r)
	}

	t.SetStyle(table.StyleLight)
	writeLine(w, t.Render())
}

// Print renders results in format: the one render switch for command output.
// table renders rows via PrintTable under fields; toon and json render v.
//
// The one-row-vs-array convention lives here: when v is a []map[string]any —
// the rows slice the caller also renders as a table — exactly one row renders
// as a single JSON object / TOON document rather than a one-element array,
// and an empty (or nil) slice renders as [] rather than null. A v that is
// anything else — a detail view whose JSON shape differs from its table row,
// or a single-row map — prints as-is.
func Print(w io.Writer, format Format, fields []string, v any, rows []map[string]any) {
	switch format {
	case FormatTable:
		PrintTable(w, fields, rows)
	case FormatToon:
		PrintToon(w, collapsed(v))
	default:
		PrintJSON(w, collapsed(v))
	}
}

// collapsed applies the one-row-vs-array convention to v. A rows slice loses
// its wrapper for zero and one rows (empty array, single object); any other
// value — a detail map, a []string, a struct — passes through unchanged.
func collapsed(v any) any {
	rows, ok := v.([]map[string]any)
	if !ok {
		return v
	}
	switch len(rows) {
	case 0:
		return []map[string]any{}
	case 1:
		return rows[0]
	default:
		return rows
	}
}

// cellValue resolves a (possibly nested) field path in a decoded row and
// formats it as a table cell. Missing keys render as empty cells; strings are
// control-stripped; numbers render without an exponent so large int64 fields
// show their digits instead of "1e+12".
func cellValue(row map[string]any, path []string) any {
	var v any = row
	for _, key := range path {
		m, ok := v.(map[string]any)
		if !ok {
			return ""
		}
		v = m[key]
	}
	switch val := v.(type) {
	case nil:
		return ""
	case string:
		return StripControl(val)
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	default:
		return fmt.Sprintf("%v", val)
	}
}

// PrintToon writes v as a TOON document (length markers on, default comma
// delimiter, 2-space indent). The value is round-tripped through JSON first
// so custom MarshalJSON implementations are honored and map keys sort
// deterministically; control bytes toon rejects are stripped before encoding.
// If toon.Marshal still fails, the JSON form is written instead — a
// data-driven marshal failure must never panic.
func PrintToon(w io.Writer, v any) {
	jsonBytes, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	var decoded any
	if err := json.Unmarshal(jsonBytes, &decoded); err != nil {
		panic(err)
	}
	toonBytes, err := toonMarshal(stripControlDeep(decoded), toon.WithLengthMarkers(true))
	if err != nil {
		writeLine(w, string(jsonBytes))
		return
	}
	writeLine(w, string(toonBytes))
}

// walkStrings returns a copy of v with f applied to every string leaf and
// every map key. It handles the shapes encoding/json unmarshals into any and
// returns non-string scalars unchanged. It never mutates the caller's input.
func walkStrings(v any, f func(string) string) any {
	switch val := v.(type) {
	case string:
		return f(val)
	case map[string]any:
		out := make(map[string]any, len(val))
		for k, item := range val {
			out[f(k)] = walkStrings(item, f)
		}
		return out
	case []any:
		out := make([]any, len(val))
		for i, item := range val {
			out[i] = walkStrings(item, f)
		}
		return out
	default:
		return v
	}
}

// stripControlDeep applies StripControl to every string value and map key in
// the shapes produced by encoding/json unmarshal-to-any. Numbers, bools and
// nil are returned unchanged.
func stripControlDeep(v any) any {
	return walkStrings(v, StripControl)
}

// writeLine writes s followed by a single newline.
func writeLine(w io.Writer, s string) {
	_, _ = io.WriteString(w, s)
	_, _ = io.WriteString(w, "\n")
}
