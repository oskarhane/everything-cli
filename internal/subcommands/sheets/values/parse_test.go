package values

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseValues(t *testing.T) {
	tests := []struct {
		name      string
		flagValue string
		fileBytes []byte
		ext       string
		want      [][]any
		wantErr   string
	}{
		// JSON flag input.
		{
			name:      "json 2d mixed scalars",
			flagValue: `[[1,"a",true],[2.5,"b",false]]`,
			want: [][]any{
				{json.Number("1"), "a", true},
				{json.Number("2.5"), "b", false},
			},
		},
		{
			name:      "json strings with empty cell",
			flagValue: `[["x",""],["y","z"]]`,
			want: [][]any{
				{"x", ""},
				{"y", "z"},
			},
		},
		{
			name:      "json single row single cell",
			flagValue: `[[42]]`,
			want:      [][]any{{json.Number("42")}},
		},
		// JSON rejections.
		{
			name:      "json scalar rejected",
			flagValue: `42`,
			wantErr:   "must be a JSON array of arrays",
		},
		{
			name:      "json 1d array rejected",
			flagValue: `[1,2,3]`,
			wantErr:   "row 0 is a number, not an array",
		},
		{
			name:      "json object rejected",
			flagValue: `{"a":1}`,
			wantErr:   "got an object",
		},
		{
			name:      "json null rejected",
			flagValue: `null`,
			wantErr:   "got null",
		},
		{
			name:      "json ragged rows rejected",
			flagValue: `[[1,2],[3]]`,
			wantErr:   "got 1 cell(s) in row 1, but row 0 has 2",
		},
		{
			name:      "json nested array cell rejected",
			flagValue: `[[1,[2]]]`,
			wantErr:   "row 0 cell 1 is an array",
		},
		{
			name:      "json object cell rejected",
			flagValue: `[["a",{"b":1}]]`,
			wantErr:   "row 0 cell 1 is an object",
		},
		{
			name:      "json null cell rejected",
			flagValue: `[[null]]`,
			wantErr:   "row 0 cell 0 is null",
		},
		{
			name:      "json invalid syntax rejected",
			flagValue: `[[1,]`,
			wantErr:   "not valid JSON",
		},
		{
			name:      "json trailing data rejected",
			flagValue: `[[1]] [[2]]`,
			wantErr:   "not valid JSON (trailing data",
		},
		{
			name:      "json empty array rejected",
			flagValue: `[]`,
			wantErr:   "at least one row",
		},
		{
			name:      "json blank flag value treated as neither",
			flagValue: "   ",
			wantErr:   "no values given",
		},
		// CSV file input.
		{
			name:      "csv basic",
			fileBytes: []byte("1,a\n2,b\n"),
			ext:       ".csv",
			want: [][]any{
				{"1", "a"},
				{"2", "b"},
			},
		},
		{
			name:      "csv quoted commas and escaped quotes",
			fileBytes: []byte("\"a,b\",\"say \"\"hi\"\"\"\nc,d\n"),
			ext:       ".csv",
			want: [][]any{
				{"a,b", `say "hi"`},
				{"c", "d"},
			},
		},
		{
			name:      "csv first row is data not header",
			fileBytes: []byte("h1,h2\n1,2\n"),
			ext:       ".csv",
			want: [][]any{
				{"h1", "h2"},
				{"1", "2"},
			},
		},
		{
			name:      "csv ragged rows rejected",
			fileBytes: []byte("a,b\nc\n"),
			ext:       ".csv",
			wantErr:   "got 1 cell(s) in row 1, but row 0 has 2",
		},
		// TSV file input.
		{
			name:      "tsv basic",
			fileBytes: []byte("1\ta\n2\tb\n"),
			ext:       ".tsv",
			want: [][]any{
				{"1", "a"},
				{"2", "b"},
			},
		},
		{
			name:      "tsv quoted field with embedded tab",
			fileBytes: []byte("\"x\ty\"\tz\n"),
			ext:       ".tsv",
			want: [][]any{
				{"x\ty", "z"},
			},
		},
		{
			name:      "tsv ragged rows rejected",
			fileBytes: []byte("a\tb\nc\n"),
			ext:       ".tsv",
			wantErr:   "same number of cells",
		},
		// JSON file input.
		{
			name:      "json file input",
			fileBytes: []byte(`[[1,"a"],[2,"b"]]`),
			ext:       ".json",
			want: [][]any{
				{json.Number("1"), "a"},
				{json.Number("2"), "b"},
			},
		},
		{
			name:      "json file 1d rejected",
			fileBytes: []byte(`[1,2]`),
			ext:       ".json",
			wantErr:   "array of arrays",
		},
		// Source selection errors.
		{
			name:      "both sources rejected",
			flagValue: `[[1]]`,
			fileBytes: []byte("1,2\n"),
			ext:       ".csv",
			wantErr:   "not both",
		},
		{
			name:    "neither source rejected",
			wantErr: "no values given",
		},
		// Empty file.
		{
			name:      "empty csv file rejected",
			fileBytes: []byte(""),
			ext:       ".csv",
			wantErr:   "values file is empty",
		},
		{
			name:      "whitespace-only csv file rejected",
			fileBytes: []byte("  \n\t"),
			ext:       ".csv",
			wantErr:   "values file is empty",
		},
		{
			name:      "empty json file rejected",
			fileBytes: []byte(""),
			ext:       ".json",
			wantErr:   "values input is empty",
		},
		// Extension handling.
		{
			name:      "unknown extension rejected",
			fileBytes: []byte("a,b\n"),
			ext:       ".xlsx",
			wantErr:   "unsupported values file extension",
		},
		{
			name:      "extension matched case-insensitively without dot",
			fileBytes: []byte("1,a\n"),
			ext:       "CSV",
			want:      [][]any{{"1", "a"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseValues(tt.flagValue, tt.fileBytes, tt.ext)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Nil(t, got)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestParseValuesLargeIntegersSurviveMarshal is the security-audit proof for
// integer precision: numbers decoded with UseNumber arrive as json.Number
// and re-marshal through encoding/json as the original literal, so
// float64-rounded values like 9.007199254740992e+15 can never reach the
// Sheets API.
func TestParseValuesLargeIntegersSurviveMarshal(t *testing.T) {
	tests := []struct {
		name      string
		flagValue string
		digit     string
	}{
		{
			name:      "2^53+1 keeps exact digits",
			flagValue: `[[9007199254740993]]`,
			digit:     "9007199254740993",
		},
		{
			name:      "18-digit id keeps exact digits",
			flagValue: `[[123456789012345678]]`,
			digit:     "123456789012345678",
		},
		{
			name:      "float literal keeps exact digits",
			flagValue: `[[2.5]]`,
			digit:     "2.5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseValues(tt.flagValue, nil, "")
			require.NoError(t, err)
			require.Equal(t, json.Number(tt.digit), got[0][0])

			out, err := json.Marshal(got)
			require.NoError(t, err)
			assert.True(t, strings.Contains(string(out), tt.digit),
				"expected %s verbatim in %s", tt.digit, out)
			assert.NotContains(t, string(out), "e+")
		})
	}
}

// TestParseValuesJSONFileLargeIntegersSurviveMarshal pins the same guarantee
// for the .json values-file path.
func TestParseValuesJSONFileLargeIntegersSurviveMarshal(t *testing.T) {
	got, err := ParseValues("", []byte(`[[123456789012345678,9007199254740993]]`), ".json")
	require.NoError(t, err)

	out, err := json.Marshal(got)
	require.NoError(t, err)
	require.JSONEq(t, `[[123456789012345678,9007199254740993]]`, string(out))
}

// TestParseValuesScientificNotationSurviveMarshal checks the literal is kept
// even for exponents the float64 path used to rewrite.
func TestParseValuesScientificNotationSurviveMarshal(t *testing.T) {
	got, err := ParseValues(`[[1e3]]`, nil, "")
	require.NoError(t, err)
	require.Equal(t, json.Number("1e3"), got[0][0])

	out, err := json.Marshal(got)
	require.NoError(t, err)
	assert.Equal(t, `[[1e3]]`, string(out))
}

// TestParseValuesNumbersAreNotFloat64 pins that the JSON path yields
// json.Number, never float64 (the corrupted-precision type).
func TestParseValuesNumbersAreNotFloat64(t *testing.T) {
	got, err := ParseValues(`[[1,2.5,true,"x"]]`, nil, "")
	require.NoError(t, err)
	for _, cell := range got[0] {
		if _, ok := cell.(json.Number); ok {
			continue
		}
		require.NotContains(t, fmt.Sprintf("%T", cell), "float64")
	}
}
