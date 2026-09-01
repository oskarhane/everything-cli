// Package values parses 2D cell values for Sheets values commands.
//
// It is pure: no cobra, no service, no filesystem access. Input arrives
// either as an inline JSON flag value or as pre-read file bytes tagged with
// a file extension. fileBytes == nil means "flag input".
package values

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"
)

// shapeExample is the canonical example used in rejection messages so users
// see the expected shape where the error occurs.
const shapeExample = `[[1,"a"],[2,"b"]]`

// extJSON, extCSV, extTSV are the supported value-file extensions.
const (
	extJSON = "json"
	extCSV  = "csv"
	extTSV  = "tsv"
)

// ParseValues parses a 2D grid of cell values from exactly one input source.
//
// Sources (XOR — exactly one must be given):
//   - flagValue: inline JSON array-of-arrays, e.g. `[[1,"a"],[2,"b"]]`
//     (pass non-empty; pass "" when using a file).
//   - fileBytes + ext: file content with ext of ".json", ".csv", or ".tsv"
//     (case-insensitive, leading dot optional). Pass fileBytes == nil when
//     using the flag.
//
// JSON values are kept typed (numbers as float64, strings, booleans);
// CSV/TSV cells are strings. Returns an error describing the expected shape
// on any malformed input.
func ParseValues(flagValue string, fileBytes []byte, ext string) ([][]any, error) {
	hasFlag := strings.TrimSpace(flagValue) != ""
	hasFile := fileBytes != nil // fileBytes == nil means flag input

	switch {
	case hasFlag && hasFile:
		return nil, fmt.Errorf("provide values from one source only: either inline JSON (array of arrays like %s) or a file, not both", shapeExample)
	case !hasFlag && !hasFile:
		return nil, fmt.Errorf("no values given: pass an inline JSON array of arrays like %s, or a .json/.csv/.tsv file", shapeExample)
	case hasFlag:
		return parseJSONValues([]byte(flagValue))
	default:
		return parseFileValues(fileBytes, ext)
	}
}

// parseFileValues dispatches on the file extension.
func parseFileValues(fileBytes []byte, ext string) ([][]any, error) {
	switch strings.TrimPrefix(strings.ToLower(strings.TrimSpace(ext)), ".") {
	case extJSON:
		return parseJSONValues(fileBytes)
	case extCSV:
		return parseDelimitedValues(fileBytes, ',')
	case extTSV:
		return parseDelimitedValues(fileBytes, '\t')
	default:
		return nil, fmt.Errorf("unsupported values file extension %q: expected .json, .csv, or .tsv", ext)
	}
}

// parseJSONValues unmarshals flagValue (or a .json file) and validates it as
// a rectangular 2D array of scalar cells.
func parseJSONValues(data []byte) ([][]any, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, fmt.Errorf("values input is empty: expected a JSON array of arrays like %s", shapeExample)
	}

	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("values is not valid JSON (%v): expected a JSON array of arrays like %s", err, shapeExample)
	}

	rows, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("values must be a JSON array of arrays like %s, got %s", shapeExample, describeJSONType(raw))
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("values must contain at least one row: expected a JSON array of arrays like %s", shapeExample)
	}

	out := make([][]any, 0, len(rows))
	width := -1
	for i, rowRaw := range rows {
		row, ok := rowRaw.([]any)
		if !ok {
			return nil, fmt.Errorf("values must be a JSON array of arrays like %s: row %d is %s, not an array", shapeExample, i, describeJSONType(rowRaw))
		}
		if width == -1 {
			width = len(row)
		} else if len(row) != width {
			return nil, fmt.Errorf("values rows must all have the same number of cells: got %d cell(s) in row %d, but row 0 has %d", len(row), i, width)
		}
		cells := make([]any, 0, len(row))
		for j, cell := range row {
			v, err := scalarCell(cell, i, j)
			if err != nil {
				return nil, err
			}
			cells = append(cells, v)
		}
		out = append(out, cells)
	}
	return out, nil
}

// scalarCell validates a JSON cell is a scalar (string, number, boolean) and
// returns it as-is.
func scalarCell(cell any, row, col int) (any, error) {
	switch cell.(type) {
	case string, float64, bool:
		return cell, nil
	case []any:
		return nil, fmt.Errorf("values cells must be scalars (string, number, or boolean): row %d cell %d is an array", row, col)
	case map[string]any:
		return nil, fmt.Errorf("values cells must be scalars (string, number, or boolean): row %d cell %d is an object", row, col)
	default:
		return nil, fmt.Errorf("values cells must be scalars (string, number, or boolean): row %d cell %d is null", row, col)
	}
}

// describeJSONType names a decoded JSON value for error messages.
func describeJSONType(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case string:
		return "a string"
	case float64:
		return "a number"
	case bool:
		return "a boolean"
	case []any:
		return "an array"
	case map[string]any:
		return "an object"
	default:
		return fmt.Sprintf("a %T", v)
	}
}

// parseDelimitedValues parses CSV (comma) or TSV (tab) content with standard
// quoting; every cell becomes a string. The first row is data — no header
// handling here. Blank lines are skipped by the CSV reader; a file with no
// rows at all is an error.
func parseDelimitedValues(fileBytes []byte, comma rune) ([][]any, error) {
	if len(bytes.TrimSpace(fileBytes)) == 0 {
		return nil, fmt.Errorf("values file is empty: expected delimited rows, one row per line")
	}

	r := csv.NewReader(bytes.NewReader(fileBytes))
	r.Comma = comma
	r.FieldsPerRecord = -1 // ragged rows are rejected by our own check below
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("could not parse values file as %s: %v", commaLabel(comma), err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("values file has no rows: expected at least one row of delimited values")
	}

	out := make([][]any, 0, len(records))
	width := -1
	for i, record := range records {
		if width == -1 {
			width = len(record)
		} else if len(record) != width {
			return nil, fmt.Errorf("values rows must all have the same number of cells: got %d cell(s) in row %d, but row 0 has %d", len(record), i, width)
		}
		row := make([]any, len(record))
		for j, cell := range record {
			row[j] = cell
		}
		out = append(out, row)
	}
	return out, nil
}

// commaLabel names a delimiter for error messages.
func commaLabel(comma rune) string {
	if comma == '\t' {
		return "tab-separated values"
	}
	return "comma-separated values"
}
