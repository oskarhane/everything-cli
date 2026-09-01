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

// TestAppendDocTextInsertsBeforeFinalNewline drives AppendDocText over a fake
// docs API: the body's last element ends at 10, so the InsertTextRequest must
// land at index 9 (inside the final paragraph, before the implicit newline).
func TestAppendDocTextInsertsAtLastEndIndexMinusOne(t *testing.T) {
	var got *docs.BatchUpdateDocumentRequest
	svc := newDocsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/v1/documents/doc-1":
			writeJSON(w, docs.Document{
				DocumentId: "doc-1",
				Body: &docs.Body{
					Content: []*docs.StructuralElement{
						{EndIndex: 5},
						{EndIndex: 10},
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

	if err := svc.AppendDocText(t.Context(), "doc-1", " the end"); err != nil {
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
	if ins.Text != " the end" {
		t.Errorf("insert text = %q, want %q", ins.Text, " the end")
	}
}

// TestAppendDocTextEmptyBody guards the error path: a body without content
// elements must fail before any batchUpdate is sent.
func TestAppendDocTextEmptyBody(t *testing.T) {
	batchUpdates := 0
	svc := newDocsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ":batchUpdate") {
			batchUpdates++
			writeJSON(w, docs.BatchUpdateDocumentResponse{})
			return
		}
		writeJSON(w, &docs.Document{DocumentId: "doc-1", Body: &docs.Body{}})
	})

	if err := svc.AppendDocText(t.Context(), "doc-1", "x"); err == nil {
		t.Fatal("AppendDocText: want error for an empty body, got nil")
	}
	if batchUpdates != 0 {
		t.Errorf("batchUpdate calls = %d, want 0 (no insert on a bad index)", batchUpdates)
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
