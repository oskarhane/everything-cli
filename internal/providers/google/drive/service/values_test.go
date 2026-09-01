package service

import (
	"net/http"
	"reflect"
	"testing"

	sheets "google.golang.org/api/sheets/v4"
)

// TestGetSpreadsheetSheetsProperties drives GetSpreadsheet over a fake
// spreadsheets.get: the request must project sheets.properties, and each
// sheet's title/sheetId/index/grid counts must round-trip into
// Spreadsheet.Sheets[].Properties.
func TestGetSpreadsheetSheetsProperties(t *testing.T) {
	svc := newDocsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v4/spreadsheets/ss-1" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if fields := r.URL.Query().Get("fields"); fields != "sheets.properties" {
			t.Errorf("fields = %q, want sheets.properties", fields)
		}
		writeJSON(w, sheets.Spreadsheet{
			SpreadsheetId: "ss-1",
			Sheets: []*sheets.Sheet{{
				Properties: &sheets.SheetProperties{
					SheetId: 123, Title: "Budget", Index: 0,
					GridProperties: &sheets.GridProperties{RowCount: 100, ColumnCount: 26},
				},
			}},
		})
	})

	ss, err := svc.GetSpreadsheet(t.Context(), "ss-1")
	if err != nil {
		t.Fatalf("GetSpreadsheet: %v", err)
	}
	if len(ss.Sheets) != 1 {
		t.Fatalf("sheets = %d, want 1", len(ss.Sheets))
	}
	props := ss.Sheets[0].Properties
	if props.SheetId != 123 || props.Title != "Budget" {
		t.Errorf("sheet properties = %+v, want sheetId 123 title Budget", props)
	}
	if props.GridProperties.RowCount != 100 || props.GridProperties.ColumnCount != 26 {
		t.Errorf("grid properties = %+v, want 100x26", props.GridProperties)
	}
}

// TestGetValuesDefaults pins the read semantics: no valueRenderOption /
// majorDimension overrides are sent (the API's defaults — formatted,
// human-readable values, major dimension ROWS) and the grid comes back as-is.
func TestGetValuesDefaults(t *testing.T) {
	svc := newDocsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v4/spreadsheets/ss-1/values/Sheet1!A1:C1" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		for _, q := range []string{"valueRenderOption", "majorDimension", "dateTimeRenderOption"} {
			if v := r.URL.Query().Get(q); v != "" {
				t.Errorf("%s = %q, want unset (API defaults)", r.URL.Query().Get("x"), v)
			}
		}
		writeJSON(w, sheets.ValueRange{
			Range:  "Sheet1!A1:C1",
			Values: [][]any{{"name", int64(42), "2026-09-01"}},
		})
	})

	got, err := svc.GetValues(t.Context(), "ss-1", "Sheet1!A1:C1")
	if err != nil {
		t.Fatalf("GetValues: %v", err)
	}
	// JSON decoding turns every number into float64, so the want grid spells
	// the number as float64 — the seam must return whatever the wire gives.
	want := [][]any{{"name", float64(42), "2026-09-01"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("values = %v, want %v", got, want)
	}
}

// TestGetValuesEmptyRange: an empty range returns an empty (nil) grid, not an
// error.
func TestGetValuesEmptyRange(t *testing.T) {
	svc := newDocsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, sheets.ValueRange{Range: "Sheet1!A1:C1"})
	})

	got, err := svc.GetValues(t.Context(), "ss-1", "Sheet1!A1:C1")
	if err != nil {
		t.Fatalf("GetValues: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("values = %v, want empty", got)
	}
}

// TestAppendValues drives values.append: the payload and valueInputOption
// must arrive as sent, and the response's updated range plus row/column
// counts come back.
func TestAppendValues(t *testing.T) {
	var gotPayload sheets.ValueRange
	svc := newDocsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v4/spreadsheets/ss-1/values/Sheet1!A1:C1:append" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if vio := r.URL.Query().Get("valueInputOption"); vio != "USER_ENTERED" {
			t.Errorf("valueInputOption = %q, want USER_ENTERED", vio)
		}
		decodeInto(t, r, &gotPayload)
		writeJSON(w, sheets.AppendValuesResponse{
			Updates: &sheets.UpdateValuesResponse{UpdatedRange: "Sheet1!A4:C4", UpdatedRows: 1, UpdatedColumns: 3},
		})
	})

	updatedRange, updatedRows, updatedCols, err := svc.AppendValues(t.Context(), "ss-1", "Sheet1!A1:C1",
		[][]any{{"a", "b", "c"}}, "USER_ENTERED")
	if err != nil {
		t.Fatalf("AppendValues: %v", err)
	}
	if updatedRange != "Sheet1!A4:C4" {
		t.Errorf("updatedRange = %q, want Sheet1!A4:C4", updatedRange)
	}
	if updatedRows != 1 || updatedCols != 3 {
		t.Errorf("updatedRows/updatedCols = %d/%d, want 1/3", updatedRows, updatedCols)
	}
	if !reflect.DeepEqual(gotPayload.Values, [][]any{{"a", "b", "c"}}) {
		t.Errorf("appended payload = %v, want [[a b c]]", gotPayload.Values)
	}
}

// TestUpdateValues drives values.update: PUT semantics, inputOption passthrough
// (RAW here), updatedCells from the response.
func TestUpdateValues(t *testing.T) {
	svc := newDocsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" || r.URL.Path != "/v4/spreadsheets/ss-1/values/Sheet1!A1:B2" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if vio := r.URL.Query().Get("valueInputOption"); vio != "RAW" {
			t.Errorf("valueInputOption = %q, want RAW", vio)
		}
		writeJSON(w, sheets.UpdateValuesResponse{UpdatedRange: "Sheet1!A1:B2", UpdatedCells: 4})
	})

	updatedRange, updatedCells, err := svc.UpdateValues(t.Context(), "ss-1", "Sheet1!A1:B2",
		[][]any{{1, 2}, {3, 4}}, "RAW")
	if err != nil {
		t.Fatalf("UpdateValues: %v", err)
	}
	if updatedRange != "Sheet1!A1:B2" || updatedCells != 4 {
		t.Errorf("got range=%q cells=%d, want Sheet1!A1:B2 / 4", updatedRange, updatedCells)
	}
}

// TestClearValues drives values.clear: POST to the :clear endpoint, and the
// response's clearedRange (which may be bounded to the sheet) comes back.
func TestClearValues(t *testing.T) {
	svc := newDocsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/v4/spreadsheets/ss-1/values/Sheet1!A1:C10:clear" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		writeJSON(w, sheets.ClearValuesResponse{ClearedRange: "Sheet1!A1:C9"})
	})

	cleared, err := svc.ClearValues(t.Context(), "ss-1", "Sheet1!A1:C10")
	if err != nil {
		t.Fatalf("ClearValues: %v", err)
	}
	if cleared != "Sheet1!A1:C9" {
		t.Errorf("clearedRange = %q, want Sheet1!A1:C9 (the server's bounded range)", cleared)
	}
}
