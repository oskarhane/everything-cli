package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrintTable(t *testing.T) {
	for _, tc := range []struct {
		name   string
		fields []string
		rows   []map[string]any
		checks []func(t *testing.T, out string)
	}{
		{
			name:   "header cells are upper-cased",
			fields: []string{"id", "from"},
			rows: []map[string]any{
				{"id": "r-1", "from": "alice@example.com"},
			},
			checks: []func(t *testing.T, out string){
				func(t *testing.T, out string) {
					// go-pretty StyleLight upper-cases header cells.
					assert.Contains(t, out, "ID")
					assert.Contains(t, out, "FROM")
					assert.NotContains(t, out, "id", "header should not stay lower-case")
					assert.NotContains(t, out, "from", "header should not stay lower-case")
				},
			},
		},
		{
			name:   "StyleLight box borders and row values",
			fields: []string{"id", "subject"},
			rows: []map[string]any{
				{"id": "1", "subject": "Hi"},
				{"id": "2", "subject": "Yo"},
			},
			checks: []func(t *testing.T, out string){
				func(t *testing.T, out string) {
					assert.Contains(t, out, "┌")
					assert.Contains(t, out, "├")
					assert.Contains(t, out, "└")
					assert.Contains(t, out, "Hi")
					assert.Contains(t, out, "Yo")
				},
			},
		},
		{
			name:   "nested field path via colon",
			fields: []string{"id", "user:name"},
			rows: []map[string]any{
				{"id": "1", "user": map[string]any{"name": "Ada"}},
			},
			checks: []func(t *testing.T, out string){
				func(t *testing.T, out string) {
					assert.Contains(t, out, "USER:NAME", "nested-path header is upper-cased too")
					assert.Contains(t, out, "Ada")
				},
			},
		},
		{
			name:   "missing key renders empty cell",
			fields: []string{"id", "missing"},
			rows:   []map[string]any{{"id": "1"}},
			checks: []func(t *testing.T, out string){
				func(t *testing.T, out string) {
					assert.Contains(t, out, "MISSING")
					assert.Contains(t, out, "│ 1  │         │", "missing key renders as an empty cell")
				},
			},
		},
		{
			name:   "nil renders empty, float64 without exponent",
			fields: []string{"note", "size"},
			rows:   []map[string]any{{"note": nil, "size": float64(1e12)}},
			checks: []func(t *testing.T, out string){
				func(t *testing.T, out string) {
					assert.Contains(t, out, "1000000000000")
					assert.NotContains(t, out, "1e+12", "large numbers must not render in scientific notation")
				},
			},
		},
		{
			name:   "control bytes stripped from string cells",
			fields: []string{"name"},
			rows:   []map[string]any{{"name": "foo\x1b[31mbar"}},
			checks: []func(t *testing.T, out string){
				func(t *testing.T, out string) {
					assert.Contains(t, out, "foo?[31mbar")
					assert.NotContains(t, out, "\x1b", "raw ESC must not reach the terminal")
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer

			PrintTable(&buf, tc.fields, tc.rows)

			out := buf.String()
			require.True(t, strings.HasSuffix(out, "\n"), "table output should end with a newline")
			for _, check := range tc.checks {
				check(t, out)
			}
		})
	}
}

// TestPrintTableExactRender pins the full StyleLight rendering of a small
// table so a style regression (borders, casing, padding) cannot pass silently.
func TestPrintTableExactRender(t *testing.T) {
	var buf bytes.Buffer

	PrintTable(&buf, []string{"id"}, []map[string]any{{"id": "1"}})

	assert.Equal(t,
		"┌────┐\n│ ID │\n├────┤\n│ 1  │\n└────┘\n",
		buf.String())
}
