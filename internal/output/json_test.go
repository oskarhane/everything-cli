package output

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrintJSON(t *testing.T) {
	for _, tc := range []struct {
		name string
		v    any
		want string
	}{
		{
			name: "map with tab indent and trailing newline",
			v:    map[string]any{"id": "abc123", "summary": "Team Sync"},
			want: "{\n\t\"id\": \"abc123\",\n\t\"summary\": \"Team Sync\"\n}\n",
		},
		{
			name: "slice of rows",
			v:    []any{map[string]any{"id": "1"}, map[string]any{"id": "2"}},
			want: "[\n\t{\n\t\t\"id\": \"1\"\n\t},\n\t{\n\t\t\"id\": \"2\"\n\t}\n]\n",
		},
		{
			name: "scalar",
			v:    42,
			want: "42\n",
		},
		{
			name: "nil",
			v:    nil,
			want: "null\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer

			PrintJSON(&buf, tc.v)

			assert.Equal(t, tc.want, buf.String())
		})
	}
}

// TestPrintJSONHonorsMarshalJSON mirrors the toon pipeline's contract: the
// printer must respect custom MarshalJSON implementations.
func TestPrintJSONHonorsMarshalJSON(t *testing.T) {
	var buf bytes.Buffer

	PrintJSON(&buf, rowEnvelope{})

	require.Contains(t, buf.String(), `"data"`)
}
