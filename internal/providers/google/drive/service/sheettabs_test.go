package service

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	sheets "google.golang.org/api/sheets/v4"
)

// tabsTestServer is the hermetic fake behind the sheet-tab service tests: a
// newDocsTestServer routed to serve one spreadsheet's sheets.properties
// metadata and to record every :batchUpdate write it receives. addID is the
// sheetId the addSheet reply echoes; writeErr makes every write fail, for
// the error-propagation paths.
type tabsTestServer struct {
	svc      *realDriveService
	sheet    *sheets.Spreadsheet // served by the metadata GET
	addID    int64               // sheetId echoed in the addSheet reply
	writeErr bool                // when set, every batchUpdate fails

	batches int                                   // batchUpdate write count
	last    *sheets.BatchUpdateSpreadsheetRequest // last decoded write
	raw     string                                // last write's raw JSON body
}

// newTabsTestServer builds the fake around spreadsheet id ss-1.
func newTabsTestServer(t *testing.T, sheet *sheets.Spreadsheet, addID int64) *tabsTestServer {
	t.Helper()
	ts := &tabsTestServer{sheet: sheet, addID: addID}
	ts.svc = newDocsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/v4/spreadsheets/ss-1":
			if fields := r.URL.Query().Get("fields"); fields != "sheets.properties" {
				t.Errorf("fields = %q, want sheets.properties", fields)
			}
			writeJSON(w, ts.sheet)
		case r.Method == "POST" && r.URL.Path == "/v4/spreadsheets/ss-1:batchUpdate":
			if ts.writeErr {
				http.Error(w, `{"error": {"code": 403, "message": "no access"}}`, http.StatusForbidden)
				return
			}
			raw, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("reading write body: %v", err)
			}
			req := &sheets.BatchUpdateSpreadsheetRequest{}
			if err := json.Unmarshal(raw, req); err != nil {
				t.Errorf("decoding write body: %v", err)
			}
			ts.batches++
			ts.last, ts.raw = req, string(raw)
			resp := sheets.BatchUpdateSpreadsheetResponse{}
			if len(req.Requests) == 1 && req.Requests[0].AddSheet != nil {
				resp.Replies = []*sheets.Response{{
					AddSheet: &sheets.AddSheetResponse{Properties: &sheets.SheetProperties{
						SheetId: ts.addID,
						Title:   req.Requests[0].AddSheet.Properties.Title,
					}},
				}}
			}
			writeJSON(w, resp)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
		}
	})
	return ts
}

// seedTabs is the metadata the tab tests serve: two grid tabs, the first
// holding the valid-but-zero sheet id.
func seedTabs() *sheets.Spreadsheet {
	return &sheets.Spreadsheet{
		SpreadsheetId: "ss-1",
		Sheets: []*sheets.Sheet{
			{Properties: &sheets.SheetProperties{SheetId: 0, Title: "Budget"}},
			{Properties: &sheets.SheetProperties{SheetId: 1, Title: "Notes"}},
		},
	}
}

// TestAddSheetTabIssuesAddSheetRequest drives AddSheetTab over the fake: one
// batchUpdate whose single request is an addSheet naming the new title, and
// the new sheet id taken from the addSheet reply.
func TestAddSheetTabIssuesAddSheetRequest(t *testing.T) {
	ts := newTabsTestServer(t, seedTabs(), 42)

	id, err := ts.svc.AddSheetTab(t.Context(), "ss-1", "Forecast")
	if err != nil {
		t.Fatalf("AddSheetTab: %v", err)
	}
	if id != 42 {
		t.Errorf("new sheet id = %d, want 42", id)
	}
	if ts.batches != 1 {
		t.Fatalf("batchUpdate calls = %d, want 1", ts.batches)
	}
	if len(ts.last.Requests) != 1 || ts.last.Requests[0].AddSheet == nil {
		t.Fatalf("request kind = %+v, want a single addSheet", ts.last.Requests)
	}
	if got := ts.last.Requests[0].AddSheet.Properties.Title; got != "Forecast" {
		t.Errorf("addSheet.properties.title = %q, want %q", got, "Forecast")
	}
}

// TestAddSheetTabPropagatesWriteError guards the error path: a failing write
// must surface wrapped, naming the title.
func TestAddSheetTabPropagatesWriteError(t *testing.T) {
	ts := newTabsTestServer(t, seedTabs(), 0)
	ts.writeErr = true

	if _, err := ts.svc.AddSheetTab(t.Context(), "ss-1", "Forecast"); err == nil {
		t.Fatal("AddSheetTab: want error on write failure, got nil")
	}
}

// TestDeleteSheetTabResolvesExactTitle drives DeleteSheetTab over the fake:
// the title is resolved through the metadata to its numeric sheetId, and the
// write is one deleteSheet carrying that id.
func TestDeleteSheetTabResolvesExactTitle(t *testing.T) {
	ts := newTabsTestServer(t, seedTabs(), 0)

	if err := ts.svc.DeleteSheetTab(t.Context(), "ss-1", "Notes"); err != nil {
		t.Fatalf("DeleteSheetTab: %v", err)
	}
	if ts.batches != 1 {
		t.Fatalf("batchUpdate calls = %d, want 1", ts.batches)
	}
	if len(ts.last.Requests) != 1 || ts.last.Requests[0].DeleteSheet == nil {
		t.Fatalf("request kind = %+v, want a single deleteSheet", ts.last.Requests)
	}
	if got := ts.last.Requests[0].DeleteSheet.SheetId; got != 1 {
		t.Errorf("deleteSheet.sheetId = %d, want 1 (Notes' id)", got)
	}
}

// TestDeleteSheetTabSendsZeroSheetID pins the omitempty hazard: the first
// tab's valid sheet id is 0, and the write must still carry it on the wire.
func TestDeleteSheetTabSendsZeroSheetID(t *testing.T) {
	ts := newTabsTestServer(t, seedTabs(), 0)

	if err := ts.svc.DeleteSheetTab(t.Context(), "ss-1", "Budget"); err != nil {
		t.Fatalf("DeleteSheetTab: %v", err)
	}
	if !strings.Contains(ts.raw, `"sheetId":0`) {
		t.Errorf("write body = %s, want an explicit sheetId 0", ts.raw)
	}
}

// TestDeleteSheetTabUnknownTitleWritesNothing: a title matching no worksheet
// (here: wrong case, proving the match is exact) must error — naming the
// title — without issuing any write.
func TestDeleteSheetTabUnknownTitleWritesNothing(t *testing.T) {
	ts := newTabsTestServer(t, seedTabs(), 0)

	err := ts.svc.DeleteSheetTab(t.Context(), "ss-1", "budget")
	if err == nil {
		t.Fatal("DeleteSheetTab: want error for an unknown title, got nil")
	}
	if !strings.Contains(err.Error(), `"budget"`) {
		t.Errorf("error = %v, want it to name the title", err)
	}
	if ts.batches != 0 {
		t.Errorf("batchUpdate calls = %d, want 0 (no write on lookup failure)", ts.batches)
	}
}

// TestRenameSheetTabIssuesUpdateSheetProperties drives RenameSheetTab over
// the fake: the write is one updateSheetProperties carrying the resolved
// sheetId, the new title, and Fields "title".
func TestRenameSheetTabIssuesUpdateSheetProperties(t *testing.T) {
	ts := newTabsTestServer(t, seedTabs(), 0)

	if err := ts.svc.RenameSheetTab(t.Context(), "ss-1", "Notes", "Archive"); err != nil {
		t.Fatalf("RenameSheetTab: %v", err)
	}
	if ts.batches != 1 {
		t.Fatalf("batchUpdate calls = %d, want 1", ts.batches)
	}
	if len(ts.last.Requests) != 1 || ts.last.Requests[0].UpdateSheetProperties == nil {
		t.Fatalf("request kind = %+v, want a single updateSheetProperties", ts.last.Requests)
	}
	up := ts.last.Requests[0].UpdateSheetProperties
	if up.Properties == nil || up.Properties.SheetId != 1 {
		t.Errorf("updateSheetProperties.properties = %+v, want sheetId 1 (Notes' id)", up.Properties)
	}
	if up.Properties.Title != "Archive" {
		t.Errorf("updateSheetProperties.properties.title = %q, want %q", up.Properties.Title, "Archive")
	}
	if up.Fields != "title" {
		t.Errorf("updateSheetProperties.fields = %q, want %q", up.Fields, "title")
	}
}

// TestRenameSheetTabUnknownTitleWritesNothing: renaming from a title that
// matches no worksheet (wrong case) must error without issuing any write.
func TestRenameSheetTabUnknownTitleWritesNothing(t *testing.T) {
	ts := newTabsTestServer(t, seedTabs(), 0)

	if err := ts.svc.RenameSheetTab(t.Context(), "ss-1", "notes", "Archive"); err == nil {
		t.Fatal("RenameSheetTab: want error for an unknown title, got nil")
	}
	if ts.batches != 0 {
		t.Errorf("batchUpdate calls = %d, want 0 (no write on lookup failure)", ts.batches)
	}
}
