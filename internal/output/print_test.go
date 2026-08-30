package output

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// decodePrintJSON unmarshals one printed JSON document.
func decodePrintJSON(t *testing.T, s string) any {
	t.Helper()
	var v any
	require.NoError(t, json.Unmarshal([]byte(s), &v))
	return v
}

func TestPrintJSONShape(t *testing.T) {
	fields := []string{"id", "snippet"}
	rows := []map[string]any{
		{"id": "r-1", "snippet": "first"},
		{"id": "r-2", "snippet": "second"},
	}

	t.Run("array", func(t *testing.T) {
		var b bytes.Buffer
		Print(&b, FormatJSON, fields, rows, rows)
		got := decodePrintJSON(t, b.String())
		arr, ok := got.([]any)
		require.True(t, ok, "expected an array, got: %s", b.String())
		require.Len(t, arr, 2)
		require.Equal(t, "r-1", arr[0].(map[string]any)["id"])
	})

	t.Run("single row collapses to an object", func(t *testing.T) {
		one := rows[:1]
		var b bytes.Buffer
		Print(&b, FormatJSON, fields, one, one)
		obj, ok := decodePrintJSON(t, b.String()).(map[string]any)
		require.True(t, ok, "expected one object, got: %s", b.String())
		require.Equal(t, "r-1", obj["id"])
	})

	t.Run("empty rows render an empty array", func(t *testing.T) {
		var b bytes.Buffer
		Print(&b, FormatJSON, fields, []map[string]any{}, nil)
		require.Equal(t, []any{}, decodePrintJSON(t, b.String()))
	})

	t.Run("nil rows slice still renders an empty array", func(t *testing.T) {
		var b bytes.Buffer
		Print(&b, FormatJSON, fields, []map[string]any(nil), nil)
		require.Equal(t, []any{}, decodePrintJSON(t, b.String()))
	})

	t.Run("non-slice detail value passes through untouched", func(t *testing.T) {
		view := map[string]any{"id": "r-1", "attendees": []map[string]any{{"email": "a@x.com"}}}
		var b bytes.Buffer
		Print(&b, FormatJSON, fields, view, rows[:1])
		obj, ok := decodePrintJSON(t, b.String()).(map[string]any)
		require.True(t, ok, "expected one object, got: %s", b.String())
		require.Equal(t, "r-1", obj["id"])
	})
}

func TestPrintToonShape(t *testing.T) {
	fields := []string{"id", "snippet"}
	rows := []map[string]any{
		{"id": "r-1", "snippet": "Hi"},
		{"id": "r-2", "snippet": "Yo"},
	}

	t.Run("array stays an array", func(t *testing.T) {
		var b bytes.Buffer
		Print(&b, FormatToon, fields, rows, rows)
		require.Contains(t, b.String(), "[#2]{", "two rows stay a length-marked TOON list")
	})

	t.Run("single row collapses to one object", func(t *testing.T) {
		one := rows[:1]
		var b bytes.Buffer
		Print(&b, FormatToon, fields, one, one)
		require.Contains(t, b.String(), "r-1")
		require.NotContains(t, b.String(), "[1]:", "a collapsed row must not be a one-element array")
	})

	t.Run("empty rows render an empty document", func(t *testing.T) {
		var b bytes.Buffer
		Print(&b, FormatToon, fields, []map[string]any{}, nil)
		require.Equal(t, "[#0]:\n", b.String(), "empty rows render a length-marked empty TOON list")
	})
}

func TestPrintTableFormat(t *testing.T) {
	fields := []string{"id", "snippet"}
	rows := []map[string]any{{"id": "r-1", "snippet": "Hi"}}
	var b bytes.Buffer
	Print(&b, FormatTable, fields, rows, rows)
	require.Contains(t, b.String(), "ID")
	require.Contains(t, b.String(), "SNIPPET")
	require.Contains(t, b.String(), "r-1")
}

func TestPrintUnknownFormatFallsBackToJSON(t *testing.T) {
	rows := []map[string]any{{"id": "r-1"}, {"id": "r-2"}}
	var b bytes.Buffer
	Print(&b, Format("yaml"), nil, rows, rows)
	require.JSONEq(t, `[{"id":"r-1"},{"id":"r-2"}]`, b.String())
}
