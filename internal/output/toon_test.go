package output

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	toon "github.com/toon-format/toon-go"
)

// rowEnvelope is a response type whose MarshalJSON wraps rows in a data
// envelope, like the API row types later nodes will define.
type rowEnvelope struct{}

func (rowEnvelope) MarshalJSON() ([]byte, error) {
	return []byte(`{"data":[{"id":"abc"}]}`), nil
}

// TestPrintToonGolden pins byte-exact TOON output for the researched format
// spec's concrete examples (all with length markers, default comma delimiter,
// 2-space indent). The expected strings are verbatim from the spec.
func TestPrintToonGolden(t *testing.T) {
	for _, tc := range []struct {
		name string
		v    any
		want string
	}{
		{
			// Example 1: MarshalJSON envelope, tabular single-column array.
			name: "marshaler envelope",
			v:    rowEnvelope{},
			want: "data[#1]{id}:\n  abc\n",
		},
		{
			// Example 2: uniform rows; numeric-looking string ids get quoted.
			name: "tabular rows with numeric-looking string ids",
			v: map[string]any{"data": []any{
				map[string]any{"id": "1", "from": "alice@example.com", "subject": "Hi"},
				map[string]any{"id": "2", "from": "bob@example.com", "subject": "Yo"},
			}},
			want: "data[#2]{from,id,subject}:\n  alice@example.com,\"1\",Hi\n  bob@example.com,\"2\",Yo\n",
		},
		{
			// Example 3: sorted keys; ':' in a value forces quoting; comma-free bare.
			name: "plain object with mixed quoting",
			v: map[string]any{
				"id":        "abc123",
				"summary":   "Team Sync",
				"start":     "2026-08-31T09:00:00+02:00",
				"attendees": []any{"a@x.com", "b@x.com"},
			},
			want: "attendees[#2]: a@x.com,b@x.com\nid: abc123\nstart: \"2026-08-31T09:00:00+02:00\"\nsummary: Team Sync\n",
		},
		{
			// Example 4: non-uniform array falls back to list form.
			name: "non-uniform array uses list form",
			v: map[string]any{"items": []any{
				map[string]any{"a": 1, "b": 2},
				map[string]any{"a": 3},
			}},
			want: "items[#2]:\n  - a: 1\n    b: 2\n  - a: 3\n",
		},
		{
			// Example 5: nested objects.
			name: "nested objects",
			v: map[string]any{
				"payload": map[string]any{
					"user":  map[string]any{"name": "Ada"},
					"roles": []any{"admin"},
				},
			},
			want: "payload:\n  roles[#1]: admin\n  user:\n    name: Ada\n",
		},
		{
			// Example 6: control bytes stripped to '?', then '[' ']' force quoting.
			name: "control bytes stripped to question marks",
			v: map[string]any{"data": []any{
				map[string]any{"name": "foo\x1b[31mbar", "note": "ring\x07bell"},
			}},
			want: "data[#1]{name,note}:\n  \"foo?[31mbar\",ring?bell\n",
		},
		{
			// Example 7: \t and \n survive stripping and render escaped.
			name: "tab and newline preserved then escaped",
			v: map[string]any{
				"body": "col1\tcol2",
				"text": "line1\nline2",
			},
			want: "body: \"col1\\tcol2\"\ntext: \"line1\\nline2\"\n",
		},
		{
			// Example 8: a comma anywhere forces quoting.
			name: "comma forces quoting",
			v:    map[string]any{"snippet": "Hello, world"},
			want: "snippet: \"Hello, world\"\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer

			PrintToon(&buf, tc.v)

			assert.Equal(t, tc.want, buf.String())
		})
	}
}

// TestPrintToonSOHByte covers the acceptance case explicitly: a 0x01 byte in
// a string value is stripped (never passed to toon, never emitted raw) and
// the marshal must not fail.
func TestPrintToonSOHByte(t *testing.T) {
	var buf bytes.Buffer

	PrintToon(&buf, map[string]any{"data": []any{
		map[string]any{"name": "a\x01b"},
	}})

	out := buf.String()
	assert.Contains(t, out, "a?b")
	assert.NotContains(t, out, "\x01")
	assert.NotContains(t, out, "error", "a stripped control byte must not trigger the JSON fallback")
}

// TestPrintToonStripsControlInKeys covers map keys, not just values.
func TestPrintToonStripsControlInKeys(t *testing.T) {
	var buf bytes.Buffer

	PrintToon(&buf, map[string]any{"a\x01b": "v"})

	// The stripped key is quoted: '?' is not an identifier rune.
	assert.Contains(t, buf.String(), "\"a?b\": v")
}

// TestPrintToonFallsBackToJSON drives the residual-error path: when
// toon.Marshal fails even after stripping, the JSON form is written instead
// and nothing panics. Values that survive the JSON round-trip are always
// toon-encodable, so the toonMarshal seam simulates the rejection.
func TestPrintToonFallsBackToJSON(t *testing.T) {
	v := map[string]any{"id": "abc", "items": []any{float64(1), float64(2)}}
	orig := toonMarshal
	toonMarshal = func(any, ...toon.EncoderOption) ([]byte, error) {
		return nil, errors.New("toon: unsupported value")
	}
	t.Cleanup(func() { toonMarshal = orig })

	var buf bytes.Buffer

	assert.NotPanics(t, func() { PrintToon(&buf, v) })

	want, err := json.Marshal(v)
	require.NoError(t, err)
	assert.Equal(t, string(want)+"\n", buf.String())
}

// TestPrintToonNotJSON asserts the neo4j-cli invariant: TOON output is not
// valid JSON and differs from the JSON printer's output for the same value.
func TestPrintToonNotJSON(t *testing.T) {
	v := map[string]any{"data": []any{
		map[string]any{"id": "1", "from": "alice@example.com", "subject": "Hi"},
	}}

	var toonBuf, jsonBuf bytes.Buffer
	PrintToon(&toonBuf, v)
	PrintJSON(&jsonBuf, v)

	assert.Error(t, json.Unmarshal(toonBuf.Bytes(), &map[string]any{}),
		"TOON output must not unmarshal as JSON")
	assert.NotEqual(t, jsonBuf.String(), toonBuf.String())

	out := toonBuf.String()
	for _, key := range []string{"data", "from", "id", "subject"} {
		assert.True(t, strings.Contains(out, key), "top-level/nested key %q must survive", key)
	}
}

// TestPrintToonEmptyDocument pins the empty-object and empty-array forms.
func TestPrintToonEmptyDocument(t *testing.T) {
	var buf bytes.Buffer

	PrintToon(&buf, map[string]any{"items": []any{}})

	// toon-go's legacy empty-array form (predates TOON spec 4.1's `key: []`).
	assert.Equal(t, "items[#0]:\n", buf.String())
}
