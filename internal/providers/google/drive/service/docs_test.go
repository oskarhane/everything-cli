package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/oauth2"
	docs "google.golang.org/api/docs/v1"
	drive "google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
	sheets "google.golang.org/api/sheets/v4"
	slides "google.golang.org/api/slides/v1"
)

// newDocsTestServer builds a hermetic fake of the whole Workspace surface the
// service wraps — drive, docs, sheets, and slides clients all pointed at one
// httptest server whose single handler routes on the request path (the A1
// ranges in sheets paths contain '!' and ':', so exact-mux patterns are
// brittle; the handler below switches on r.URL.Path instead). It is the
// shared seam-level fake for the docs, values, and slides tests.
func newDocsTestServer(t *testing.T, handler http.HandlerFunc) *realDriveService {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	opts := []option.ClientOption{
		option.WithEndpoint(srv.URL),
		option.WithTokenSource(oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test-token"})),
	}
	driveSvc, err := drive.NewService(t.Context(), opts...)
	if err != nil {
		t.Fatalf("creating drive service: %v", err)
	}
	docsSvc, err := docs.NewService(t.Context(), opts...)
	if err != nil {
		t.Fatalf("creating docs service: %v", err)
	}
	sheetsSvc, err := sheets.NewService(t.Context(), opts...)
	if err != nil {
		t.Fatalf("creating sheets service: %v", err)
	}
	slidesSvc, err := slides.NewService(t.Context(), opts...)
	if err != nil {
		t.Fatalf("creating slides service: %v", err)
	}
	return &realDriveService{drive: driveSvc, docs: docsSvc, sheets: sheetsSvc, slides: slidesSvc}
}

// decodeInto reads the request's JSON body into v (tests that inspect what
// the client sent).
func decodeInto(t *testing.T, r *http.Request, v any) {
	t.Helper()
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		t.Errorf("decoding request body: %v", err)
	}
}

// TestEndBodyIndex covers the append-index rule: the index is the last
// body.content element's exclusive endIndex minus 1 — the position of the
// body's implicit final newline — and nil/empty bodies or elements without an
// endIndex must error rather than produce a bogus index.
func TestEndBodyIndex(t *testing.T) {
	tests := []struct {
		name    string
		body    *docs.Body
		want    int64
		wantErr bool
	}{
		{
			name:    "nil body errors",
			body:    nil,
			wantErr: true,
		},
		{
			name:    "empty content errors",
			body:    &docs.Body{},
			wantErr: true,
		},
		{
			name: "single paragraph ending in final newline",
			// "Hello\n" occupies indexes 0..6 (newline at 6); endIndex 7 is
			// the segment end the API forbids inserting at.
			body: &docs.Body{Content: []*docs.StructuralElement{{EndIndex: 7}}},
			want: 6,
		},
		{
			name: "append index is the last element's endIndex minus 1",
			body: &docs.Body{Content: []*docs.StructuralElement{
				{EndIndex: 6},
				{EndIndex: 10},
				{EndIndex: 25},
			}},
			want: 24,
		},
		{
			name: "last element without endIndex errors",
			body: &docs.Body{Content: []*docs.StructuralElement{
				{EndIndex: 10},
				{EndIndex: 0},
			}},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := endBodyIndex(tt.body)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("endBodyIndex(%+v) = %d, want error", tt.body, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("endBodyIndex(%+v): %v", tt.body, err)
			}
			if got != tt.want {
				t.Errorf("endBodyIndex(%+v) = %d, want %d", tt.body, got, tt.want)
			}
		})
	}
}

// TestAppendDocTextInsertsAtLastEndIndexMinusOne drives AppendDocText over a fake
// docs API: the tab body's last element ends at 10, so the InsertTextRequest
// must land at index 9 (inside the final paragraph, before the implicit
// newline) of the first tab, which an empty tabID targets.
func TestAppendDocTextInsertsAtLastEndIndexMinusOne(t *testing.T) {
	var got *docs.BatchUpdateDocumentRequest
	svc := newDocsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/v1/documents/doc-1":
			writeJSON(w, docs.Document{
				DocumentId: "doc-1",
				Tabs: []*docs.Tab{
					tabEndingAt("t.Main", "Notes", 10),
				},
			})
		case r.Method == "POST" && r.URL.Path == "/v1/documents/doc-1:batchUpdate":
			req := &docs.BatchUpdateDocumentRequest{}
			decodeInto(t, r, req)
			got = req
			writeJSON(w, &docs.BatchUpdateDocumentResponse{DocumentId: "doc-1"})
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
			http.Error(w, "not found", http.StatusNotFound)
		}
	})

	if err := svc.AppendDocText(t.Context(), "doc-1", " the end", ""); err != nil {
		t.Fatalf("AppendDocText: %v", err)
	}
	if len(got.Requests) != 1 {
		t.Fatalf("batchUpdate sent %d requests, want 1", len(got.Requests))
	}
	ins := got.Requests[0].InsertText
	if ins == nil {
		t.Fatalf("request kind = %+v, want a single insertText", got.Requests[0])
	}
	if ins.Location == nil || ins.Location.Index != 9 {
		t.Fatalf("insert index = %+v, want 9 (last endIndex 10 - 1, before the final newline)", ins.Location)
	}
	if ins.Location.TabId != "" {
		t.Errorf("location.tabId = %q, want empty (empty tabID lets the API pick the first tab)", ins.Location.TabId)
	}
	if ins.Text != " the end" {
		t.Errorf("insert text = %q, want %q", ins.Text, " the end")
	}
}

// TestAppendDocTextEmptyBody guards the error path: a tab body without
// content elements must fail before any batchUpdate is sent.
func TestAppendDocTextEmptyBody(t *testing.T) {
	batchUpdates := 0
	svc := newDocsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ":batchUpdate") {
			batchUpdates++
			writeJSON(w, docs.BatchUpdateDocumentResponse{})
			return
		}
		writeJSON(w, &docs.Document{DocumentId: "doc-1", Tabs: []*docs.Tab{{
			TabProperties: &docs.TabProperties{TabId: "t.Main", Title: "Notes"},
			DocumentTab:   &docs.DocumentTab{Body: &docs.Body{}},
		}}})
	})

	if err := svc.AppendDocText(t.Context(), "doc-1", "x", ""); err == nil {
		t.Fatal("AppendDocText: want error for an empty body, got nil")
	}
	if batchUpdates != 0 {
		t.Errorf("batchUpdate calls = %d, want 0 (no insert on a bad index)", batchUpdates)
	}
}

// TestInsertDocTextForwardsIndex drives InsertDocText over a fake batchUpdate:
// the request must be exactly one insertText whose location.index is the
// caller's index, forwarded untouched.
func TestInsertDocTextForwardsIndex(t *testing.T) {
	var got *docs.BatchUpdateDocumentRequest
	svc := newDocsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/v1/documents/doc-1:batchUpdate" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		req := &docs.BatchUpdateDocumentRequest{}
		decodeInto(t, r, req)
		got = req
		writeJSON(w, &docs.BatchUpdateDocumentResponse{DocumentId: "doc-1"})
	})

	if err := svc.InsertDocText(t.Context(), "doc-1", "up front", 1, ""); err != nil {
		t.Fatalf("InsertDocText: %v", err)
	}
	if len(got.Requests) != 1 {
		t.Fatalf("batchUpdate sent %d requests, want 1", len(got.Requests))
	}
	ins := got.Requests[0].InsertText
	if ins == nil {
		t.Fatalf("request kind = %+v, want a single insertText", got.Requests[0])
	}
	if ins.Location == nil || ins.Location.Index != 1 {
		t.Fatalf("insert index = %+v, want 1 (the caller's index, forwarded as-is)", ins.Location)
	}
	if ins.Text != "up front" {
		t.Errorf("insert text = %q, want %q", ins.Text, "up front")
	}
}

// TestInsertDocTextPropagatesAPIError guards the error path: an API failure
// must surface wrapped, naming the document.
func TestInsertDocTextPropagatesAPIError(t *testing.T) {
	svc := newDocsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error": {"code": 403, "message": "no access"}}`, http.StatusForbidden)
	})

	if err := svc.InsertDocText(t.Context(), "doc-1", "x", 1, ""); err == nil {
		t.Fatal("InsertDocText: want error on API failure, got nil")
	}
}

// TestInsertDocTextForwardsTabID drives InsertDocText with a tab key: the
// request must pin the insert to that tab (an empty tabID is the API's own
// first-tab default, covered by TestInsertDocTextForwardsIndex).
func TestInsertDocTextForwardsTabID(t *testing.T) {
	var got *docs.BatchUpdateDocumentRequest
	svc := newDocsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/v1/documents/doc-1:batchUpdate" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		req := &docs.BatchUpdateDocumentRequest{}
		decodeInto(t, r, req)
		got = req
		writeJSON(w, &docs.BatchUpdateDocumentResponse{DocumentId: "doc-1"})
	})

	if err := svc.InsertDocText(t.Context(), "doc-1", "tabbed", 7, "t.Child"); err != nil {
		t.Fatalf("InsertDocText: %v", err)
	}
	ins := got.Requests[0].InsertText
	if ins == nil || ins.Location == nil || ins.Location.Index != 7 {
		t.Fatalf("insert location = %+v, want index 7 forwarded untouched", ins)
	}
	if ins.Location.TabId != "t.Child" {
		t.Errorf("location.tabId = %q, want t.Child", ins.Location.TabId)
	}
}

// TestReplaceDocText drives ReplaceDocText over a fake batchUpdate: the
// request must carry containsText (text + matchCase) and replaceText, and the
// reply's occurrencesChanged must come back as the count.
func TestReplaceDocText(t *testing.T) {
	var got *docs.BatchUpdateDocumentRequest
	svc := newDocsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/v1/documents/doc-1:batchUpdate" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		req := &docs.BatchUpdateDocumentRequest{}
		decodeInto(t, r, req)
		got = req
		writeJSON(w, docs.BatchUpdateDocumentResponse{
			Replies: []*docs.Response{{ReplaceAllText: &docs.ReplaceAllTextResponse{OccurrencesChanged: 3}}},
		})
	})

	count, err := svc.ReplaceDocText(t.Context(), "doc-1", "teh", "the", false)
	if err != nil {
		t.Fatalf("ReplaceDocText: %v", err)
	}
	if count != 3 {
		t.Errorf("occurrences changed = %d, want 3", count)
	}
	if len(got.Requests) != 1 || got.Requests[0].ReplaceAllText == nil {
		t.Fatalf("request kind = %+v, want a single replaceAllText", got.Requests[0])
	}
	rq := got.Requests[0].ReplaceAllText
	if rq.ReplaceText != "the" {
		t.Errorf("replaceText = %q, want %q", rq.ReplaceText, "the")
	}
	if rq.ContainsText == nil {
		t.Fatal("containsText missing from the request")
	}
	if rq.ContainsText.Text != "teh" {
		t.Errorf("containsText.text = %q, want %q", rq.ContainsText.Text, "teh")
	}
	if rq.ContainsText.MatchCase {
		t.Error("containsText.matchCase = true, want false (caller passed matchCase=false)")
	}
}

// TestReplaceDocTextMatchCase passes the matchCase flag through to the
// criteria instead of forcing the default.
func TestReplaceDocTextMatchCasePassthrough(t *testing.T) {
	var sawMatchCase bool
	svc := newDocsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		req := &docs.BatchUpdateDocumentRequest{}
		decodeInto(t, r, req)
		sawMatchCase = req.Requests[0].ReplaceAllText.ContainsText.MatchCase
		writeJSON(w, docs.BatchUpdateDocumentResponse{})
	})

	if _, err := svc.ReplaceDocText(t.Context(), "doc-1", "Foo", "Bar", true); err != nil {
		t.Fatalf("ReplaceDocText: %v", err)
	}
	if !sawMatchCase {
		t.Error("containsText.matchCase = false, want the caller's true")
	}
}

// TestGetDocTextStreamsExport drives GetDocText over the drive export
// endpoint: the whole body must come back verbatim, and the request must ask
// for the text/plain export mime type.
func TestGetDocTextStreamsExport(t *testing.T) {
	var sawMime string
	svc := newDocsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/files/doc-1/export" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		sawMime = r.URL.Query().Get("mimeType")
		_, _ = w.Write([]byte("Hello\nworld\n"))
	})

	text, err := svc.GetDocText(t.Context(), "doc-1")
	if err != nil {
		t.Fatalf("GetDocText: %v", err)
	}
	if sawMime != textExportMime {
		t.Errorf("export mimeType = %q, want %q", sawMime, textExportMime)
	}
	if text != "Hello\nworld\n" {
		t.Errorf("text = %q, want the full export verbatim", text)
	}
}

// tabEndingAt builds a tab whose body's single content element ends at
// lastEndIndex — the value AppendDocText's index math drives off.
func tabEndingAt(id, title string, lastEndIndex int64) *docs.Tab {
	return &docs.Tab{
		TabProperties: &docs.TabProperties{TabId: id, Title: title},
		DocumentTab: &docs.DocumentTab{Body: &docs.Body{
			Content: []*docs.StructuralElement{{EndIndex: lastEndIndex}},
		}},
	}
}

// TestAppendDocTextTargetsFirstTab proves the multi-tab append fix: the read
// must ask for includeTabsContent=true (the legacy top-level body is empty
// for multi-tab documents), the chosen tab must be the first root tab, and
// the insert must land at that tab's body end — not the second tab's.
func TestAppendDocTextTargetsFirstTab(t *testing.T) {
	var got *docs.BatchUpdateDocumentRequest
	var sawInclude, sawFields string
	svc := newDocsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/v1/documents/doc-1":
			sawInclude = r.URL.Query().Get("includeTabsContent")
			sawFields = r.URL.Query().Get("fields")
			writeJSON(w, docs.Document{
				DocumentId: "doc-1",
				Tabs: []*docs.Tab{
					tabEndingAt("t.First", "Notes", 33),
					tabEndingAt("t.Second", "Archive", 50),
				},
			})
		case r.Method == "POST" && r.URL.Path == "/v1/documents/doc-1:batchUpdate":
			req := &docs.BatchUpdateDocumentRequest{}
			decodeInto(t, r, req)
			got = req
			writeJSON(w, &docs.BatchUpdateDocumentResponse{DocumentId: "doc-1"})
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
			http.Error(w, "not found", http.StatusNotFound)
		}
	})

	if err := svc.AppendDocText(t.Context(), "doc-1", " more", ""); err != nil {
		t.Fatalf("AppendDocText: %v", err)
	}
	if sawInclude != "true" {
		t.Errorf("includeTabsContent = %q, want true (the legacy body is empty for multi-tab docs)", sawInclude)
	}
	if sawFields != tabsFields {
		t.Errorf("fields = %q, want %q", sawFields, tabsFields)
	}
	ins := got.Requests[0].InsertText
	if ins.Location == nil || ins.Location.Index != 32 {
		t.Fatalf("insert index = %+v, want 32 (the FIRST tab's last endIndex 33 - 1)", ins.Location)
	}
	if ins.Location.TabId != "" {
		t.Errorf("location.tabId = %q, want empty for an empty tabID", ins.Location.TabId)
	}
}

// TestAppendDocTextTargetsRequestedTab drives AppendDocText with a tab key:
// the insert must be pinned to the resolved tab (by ID or title — the
// location carries the resolved tab's ID either way) and indexed relative to
// that tab's own body end, not the first tab's.
func TestAppendDocTextTargetsRequestedTab(t *testing.T) {
	tests := []struct {
		name      string
		key       string
		wantTabID string
		wantIndex int64
	}{
		{name: "by tab ID", key: "t.Child", wantTabID: "t.Child", wantIndex: 39},
		{name: "by title resolves to the tab's ID", key: "Archive", wantTabID: "t.Child", wantIndex: 39},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got *docs.BatchUpdateDocumentRequest
			svc := newDocsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == "GET" && r.URL.Path == "/v1/documents/doc-1":
					writeJSON(w, docs.Document{
						DocumentId: "doc-1",
						Tabs: []*docs.Tab{
							tabEndingAt("t.Root", "Notes", 20),
							{
								TabProperties: &docs.TabProperties{TabId: "t.Child", Title: "Archive"},
								DocumentTab: &docs.DocumentTab{Body: &docs.Body{
									Content: []*docs.StructuralElement{{EndIndex: 40}},
								}},
							},
						},
					})
				case r.Method == "POST" && r.URL.Path == "/v1/documents/doc-1:batchUpdate":
					req := &docs.BatchUpdateDocumentRequest{}
					decodeInto(t, r, req)
					got = req
					writeJSON(w, &docs.BatchUpdateDocumentResponse{DocumentId: "doc-1"})
				default:
					t.Errorf("unexpected request %s %s", r.Method, r.URL)
					http.Error(w, "not found", http.StatusNotFound)
				}
			})

			if err := svc.AppendDocText(t.Context(), "doc-1", " more", tt.key); err != nil {
				t.Fatalf("AppendDocText: %v", err)
			}
			ins := got.Requests[0].InsertText
			if ins.Location == nil || ins.Location.Index != tt.wantIndex {
				t.Fatalf("insert index = %+v, want %d (the requested tab's own body end)", ins.Location, tt.wantIndex)
			}
			if ins.Location.TabId != tt.wantTabID {
				t.Errorf("location.tabId = %q, want %q", ins.Location.TabId, tt.wantTabID)
			}
		})
	}
}

// TestListDocTabsFlattensDepthFirst drives ListDocTabs over a seeded tree:
// the list must read parent-before-child (recursively) and carry each tab's
// properties under the output-facing snake_case keys.
func TestListDocTabsFlattensDepthFirst(t *testing.T) {
	svc := newDocsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/v1/documents/doc-1" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		writeJSON(w, docs.Document{
			DocumentId: "doc-1",
			Tabs: []*docs.Tab{
				tabEndingAt("t.A", "Notes", 10),
				{
					TabProperties: &docs.TabProperties{TabId: "t.B", Title: "Archive", Index: 1},
					ChildTabs: []*docs.Tab{{
						TabProperties: &docs.TabProperties{
							TabId: "t.C", Title: "Old notes", Index: 0, NestingLevel: 1, ParentTabId: "t.B",
						},
						ChildTabs: []*docs.Tab{{
							TabProperties: &docs.TabProperties{
								TabId: "t.D", Title: "Deeper", Index: 0, NestingLevel: 2, ParentTabId: "t.C",
							},
						}},
					}},
				},
			},
		})
	})

	tabs, err := svc.ListDocTabs(t.Context(), "doc-1")
	if err != nil {
		t.Fatalf("ListDocTabs: %v", err)
	}
	wantOrder := []string{"t.A", "t.B", "t.C", "t.D"}
	if len(tabs) != len(wantOrder) {
		t.Fatalf("ListDocTabs returned %d tabs, want %d", len(tabs), len(wantOrder))
	}
	for i, want := range wantOrder {
		if tabs[i].TabID != want {
			t.Errorf("tabs[%d].TabID = %q, want %q (depth-first order)", i, tabs[i].TabID, want)
		}
	}
	child := tabs[2]
	if child.Title != "Old notes" || child.NestingLevel != 1 || child.ParentTabID != "t.B" || child.Index != 0 {
		t.Errorf("nested child tab = %+v, want the seeded t.C properties", child)
	}
	b, err := json.Marshal(tabs[0])
	if err != nil {
		t.Fatalf("marshaling DocTab: %v", err)
	}
	for _, key := range []string{`"tab_id"`, `"title"`, `"index"`, `"nesting_level"`, `"parent_tab_id"`} {
		if !strings.Contains(string(b), key) {
			t.Errorf("DocTab JSON %s missing key %s (output keys are snake_case)", b, key)
		}
	}
}

// TestGetDocTabTextRendersParagraphsAndTables drives GetDocTabText over a
// seeded tab body with both element kinds it keeps: text-run paragraphs and
// table cells, one line per paragraph.
func TestGetDocTabTextRendersParagraphsAndTables(t *testing.T) {
	svc := newDocsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/v1/documents/doc-1" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		writeJSON(w, seedTabBodyDoc())
	})

	text, err := svc.GetDocTabText(t.Context(), "doc-1", "t.Main")
	if err != nil {
		t.Fatalf("GetDocTabText: %v", err)
	}
	want := "Hello world\nCell A\nCell B two\n"
	if text != want {
		t.Errorf("GetDocTabText = %q, want %q (one line per paragraph, cells flattened in)", text, want)
	}
}

// TestTabWithoutDocumentTabErrors proves a malformed tab response (no
// documentTab body) errors on both read and write instead of nil-panicking.
func TestTabWithoutDocumentTabErrors(t *testing.T) {
	svc := newDocsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/v1/documents/doc-1" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		writeJSON(w, &docs.Document{
			DocumentId: "doc-1",
			Tabs:       []*docs.Tab{{TabProperties: &docs.TabProperties{TabId: "t.Main", Title: "Notes"}}},
		})
	})

	if _, err := svc.GetDocTabText(t.Context(), "doc-1", "t.Main"); err == nil {
		t.Error("GetDocTabText: want error for a tab with no documentTab body")
	}
	if err := svc.AppendDocText(t.Context(), "doc-1", "text", "t.Main"); err == nil {
		t.Error("AppendDocText: want error for a tab with no documentTab body")
	}
}

// seedTabBodyDoc returns a one-tab document whose body holds both element
// kinds GetDocTabText keeps: a text-run paragraph and a one-row table.
func seedTabBodyDoc() *docs.Document {
	para := func(runs ...string) *docs.Paragraph {
		els := make([]*docs.ParagraphElement, 0, len(runs))
		for _, run := range runs {
			els = append(els, &docs.ParagraphElement{TextRun: &docs.TextRun{Content: run}})
		}
		return &docs.Paragraph{Elements: els}
	}
	return &docs.Document{
		DocumentId: "doc-1",
		Tabs: []*docs.Tab{{
			TabProperties: &docs.TabProperties{TabId: "t.Main", Title: "Notes"},
			DocumentTab: &docs.DocumentTab{Body: &docs.Body{Content: []*docs.StructuralElement{
				// The API ships a paragraph's newline inside its final run.
				{Paragraph: para("Hello ", "world\n")},
				{Table: &docs.Table{TableRows: []*docs.TableRow{{
					TableCells: []*docs.TableCell{
						{Content: []*docs.StructuralElement{{Paragraph: para("Cell A\n")}}},
						{Content: []*docs.StructuralElement{{Paragraph: para("Cell B", " two")}}},
					},
				}}}},
			}}},
		}},
	}
}

// TestChooseTab covers the shared tab lookup: an empty key picks the first
// tab; an exact tab ID wins over a same-named title; titles resolve inside
// nested child tabs; an ambiguous title errors naming both matches; unknown
// keys error.
func TestChooseTab(t *testing.T) {
	tests := []struct {
		name    string
		tabs    []*docs.Tab
		key     string
		wantID  string
		wantErr []string // substrings the error must carry
	}{
		{
			name:   "empty key picks the first root tab",
			tabs:   []*docs.Tab{tabEndingAt("t.A", "Notes", 10), tabEndingAt("t.B", "Archive", 10)},
			key:    "",
			wantID: "t.A",
		},
		{
			name: "exact tab ID wins over a same-named title",
			tabs: []*docs.Tab{
				tabEndingAt("t.A", "Shared", 10),
				tabEndingAt("Shared", "Other", 10),
			},
			key:    "Shared",
			wantID: "Shared",
		},
		{
			name: "matches a nested child tab by ID",
			tabs: []*docs.Tab{{
				TabProperties: &docs.TabProperties{TabId: "t.Root", Title: "Notes"},
				ChildTabs: []*docs.Tab{{
					TabProperties: &docs.TabProperties{TabId: "t.Child", Title: "Archive"},
				}},
			}},
			key:    "t.Child",
			wantID: "t.Child",
		},
		{
			name: "matches a nested child tab by title",
			tabs: []*docs.Tab{{
				TabProperties: &docs.TabProperties{TabId: "t.Root", Title: "Notes"},
				ChildTabs: []*docs.Tab{{
					TabProperties: &docs.TabProperties{TabId: "t.Child", Title: "Archive"},
				}},
			}},
			key:    "Archive",
			wantID: "t.Child",
		},
		{
			name: "ambiguous title errors naming both matches",
			tabs: []*docs.Tab{{
				TabProperties: &docs.TabProperties{TabId: "t.Root", Title: "Shared"},
				ChildTabs: []*docs.Tab{{
					TabProperties: &docs.TabProperties{TabId: "t.Child", Title: "Shared"},
				}},
			}},
			key:     "Shared",
			wantErr: []string{"t.Root", "t.Child"},
		},
		{
			name:    "unknown key errors",
			tabs:    []*docs.Tab{tabEndingAt("t.A", "Notes", 10)},
			key:     "nope",
			wantErr: []string{`no tab with ID or title "nope"`},
		},
		{
			name:    "document with no tabs errors",
			tabs:    nil,
			key:     "",
			wantErr: []string{"document has no tabs"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tab, err := chooseTab(&docs.Document{Tabs: tt.tabs}, tt.key)
			if len(tt.wantErr) > 0 {
				if err == nil {
					t.Fatalf("chooseTab(%q) = %+v, want error", tt.key, tab)
				}
				for _, frag := range tt.wantErr {
					if !strings.Contains(err.Error(), frag) {
						t.Errorf("chooseTab(%q) error %q missing %q", tt.key, err, frag)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("chooseTab(%q): %v", tt.key, err)
			}
			if tab.TabProperties.TabId != tt.wantID {
				t.Errorf("chooseTab(%q) = tab %q, want %q", tt.key, tab.TabProperties.TabId, tt.wantID)
			}
		})
	}
}

// TestAddDocTabReturnsReplyTabID drives AddDocTab: the request adds a tab
// with the given title, and the new tab's ID comes back from the reply.
func TestAddDocTabReturnsReplyTabID(t *testing.T) {
	var got *docs.BatchUpdateDocumentRequest
	svc := newDocsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/v1/documents/doc-1:batchUpdate" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		req := &docs.BatchUpdateDocumentRequest{}
		decodeInto(t, r, req)
		got = req
		writeJSON(w, docs.BatchUpdateDocumentResponse{
			Replies: []*docs.Response{{AddDocumentTab: &docs.AddDocumentTabResponse{
				// The reply reports the created tab's properties; the ID
				// rides inside them.
				TabProperties: &docs.TabProperties{TabId: "t.New", Title: "Plan"},
			}}},
		})
	})

	id, err := svc.AddDocTab(t.Context(), "doc-1", "Plan")
	if err != nil {
		t.Fatalf("AddDocTab: %v", err)
	}
	if id != "t.New" {
		t.Errorf("AddDocTab id = %q, want t.New", id)
	}
	add := got.Requests[0].AddDocumentTab
	if add == nil || add.TabProperties == nil || add.TabProperties.Title != "Plan" {
		t.Fatalf("request kind = %+v, want one addDocumentTab titled Plan", got.Requests[0])
	}
}

// TestAddDocTabErrorsWithoutReplyTabID guards the reply parse: a reply
// without a usable tab ID must error, not return an empty ID.
func TestAddDocTabErrorsWithoutReplyTabID(t *testing.T) {
	svc := newDocsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, docs.BatchUpdateDocumentResponse{
			Replies: []*docs.Response{{AddDocumentTab: &docs.AddDocumentTabResponse{}}},
		})
	})

	if _, err := svc.AddDocTab(t.Context(), "doc-1", "Plan"); err == nil {
		t.Fatal("AddDocTab: want error when the reply carries no tab ID, got nil")
	}
}

// TestRenameDocTabSendsTitleMask drives RenameDocTab: the request must carry
// the tab's ID and new title, and the mask must cover "title" only — the
// root tab_properties is implied and must not be listed.
func TestRenameDocTabSendsTitleMask(t *testing.T) {
	var got *docs.BatchUpdateDocumentRequest
	svc := newDocsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		req := &docs.BatchUpdateDocumentRequest{}
		decodeInto(t, r, req)
		got = req
		writeJSON(w, &docs.BatchUpdateDocumentResponse{DocumentId: "doc-1"})
	})

	if err := svc.RenameDocTab(t.Context(), "doc-1", "t.B", "Renamed"); err != nil {
		t.Fatalf("RenameDocTab: %v", err)
	}
	upd := got.Requests[0].UpdateDocumentTabProperties
	if upd == nil {
		t.Fatalf("request kind = %+v, want one updateDocumentTabProperties", got.Requests[0])
	}
	if upd.TabProperties == nil || upd.TabProperties.TabId != "t.B" || upd.TabProperties.Title != "Renamed" {
		t.Errorf("tabProperties = %+v, want {t.B, Renamed}", upd.TabProperties)
	}
	if upd.Fields != "title" {
		t.Errorf("fields = %q, want exactly %q (the root tab_properties is implied)", upd.Fields, "title")
	}
}

// TestDeleteDocTabSendsTabID drives DeleteDocTab: the request must delete
// exactly the given tab (the API takes its child tabs with it).
func TestDeleteDocTabSendsTabID(t *testing.T) {
	var got *docs.BatchUpdateDocumentRequest
	svc := newDocsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		req := &docs.BatchUpdateDocumentRequest{}
		decodeInto(t, r, req)
		got = req
		writeJSON(w, &docs.BatchUpdateDocumentResponse{DocumentId: "doc-1"})
	})

	if err := svc.DeleteDocTab(t.Context(), "doc-1", "t.B"); err != nil {
		t.Fatalf("DeleteDocTab: %v", err)
	}
	del := got.Requests[0].DeleteTab
	if del == nil || del.TabId != "t.B" {
		t.Fatalf("request kind = %+v, want one deleteTab for t.B", got.Requests[0])
	}
}
