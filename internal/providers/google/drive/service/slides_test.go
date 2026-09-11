package service

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	slides "google.golang.org/api/slides/v1"
)

// readBody reads a request body as a string so a test can assert on the raw
// wire shape (e.g. a force-sent zero-valued field) as well as the decoded
// form.
func readBody(t *testing.T, r *http.Request) string {
	t.Helper()
	b, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("reading request body: %v", err)
	}
	return string(b)
}

// TestGetSlideText drives GetSlideText over a fake presentations.get: every
// shape with non-empty text comes back with its 1-based slide number and
// object ID, shapes without text (or without shapes at all) are skipped, and
// text runs join in order.
func TestGetSlideText(t *testing.T) {
	svc := newDocsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/presentations/p-1" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		writeJSON(w, slides.Presentation{
			PresentationId: "p-1",
			Slides: []*slides.Page{
				{PageElements: []*slides.PageElement{
					{ObjectId: "title-shape", Shape: &slides.Shape{Text: &slides.TextContent{
						TextElements: []*slides.TextElement{
							{TextRun: &slides.TextRun{Content: "Hello "}},
							{TextRun: &slides.TextRun{Content: "World"}},
						},
					}}},
					{ObjectId: "image-only"}, // no shape text
				},
				},
				{PageElements: []*slides.PageElement{
					{ObjectId: "empty-shape", Shape: &slides.Shape{Text: &slides.TextContent{}}},
					{ObjectId: "body-shape", Shape: &slides.Shape{Text: &slides.TextContent{
						TextElements: []*slides.TextElement{{TextRun: &slides.TextRun{Content: "second\nslide"}}},
					}}},
				}},
			},
		})
	})

	shapes, err := svc.GetSlideText(t.Context(), "p-1")
	if err != nil {
		t.Fatalf("GetSlideText: %v", err)
	}
	want := []SlideShape{
		{SlideNumber: 1, ShapeID: "title-shape", Text: "Hello World"},
		{SlideNumber: 2, ShapeID: "body-shape", Text: "second\nslide"},
	}
	if len(shapes) != len(want) {
		t.Fatalf("shapes = %+v, want %d shapes (text-bearing only)", shapes, len(want))
	}
	for i, w := range want {
		if shapes[i] != w {
			t.Errorf("shapes[%d] = %+v, want %+v", i, shapes[i], w)
		}
	}
}

// TestGetSlideTextErrors propagates API failures with the presentation ID.
func TestGetSlideTextError(t *testing.T) {
	svc := newDocsTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})

	if _, err := svc.GetSlideText(t.Context(), "p-1"); err == nil {
		t.Fatal("GetSlideText: want error from a failing endpoint, got nil")
	}
}

// TestReplaceSlideText drives ReplaceSlideText over a fake batchUpdate: one
// request, criteria + replacement + matchCase as sent, and the reply's
// occurrencesChanged returned as the count.
func TestReplaceSlideText(t *testing.T) {
	var got *slides.BatchUpdatePresentationRequest
	svc := newDocsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/v1/presentations/p-1:batchUpdate" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		req := &slides.BatchUpdatePresentationRequest{}
		decodeInto(t, r, req)
		got = req
		writeJSON(w, slides.BatchUpdatePresentationResponse{
			Replies: []*slides.Response{{ReplaceAllText: &slides.ReplaceAllTextResponse{OccurrencesChanged: 3}}},
		})
	})

	count, err := svc.ReplaceSlideText(t.Context(), "p-1", "2025", "2026", false)
	if err != nil {
		t.Fatalf("ReplaceSlideText: %v", err)
	}
	if count != 3 {
		t.Errorf("occurrences changed = %d, want 3", count)
	}
	if len(got.Requests) != 1 || got.Requests[0].ReplaceAllText == nil {
		t.Fatalf("request kind = %+v, want a single replaceAllText", got.Requests[0])
	}
	rq := got.Requests[0].ReplaceAllText
	if rq.ReplaceText != "2026" {
		t.Errorf("replaceText = %q, want %q", rq.ReplaceText, "2026")
	}
	if rq.ContainsText == nil || rq.ContainsText.Text != "2025" || rq.ContainsText.MatchCase {
		t.Fatalf("containsText = %+v, want {text: 2025, matchCase: false}", rq.ContainsText)
	}
}

// TestReplaceSlideTextSumsPerSlideReplies guards the count aggregation: the
// server may answer with one reply per request, and each reply's
// occurrencesChanged must be summed rather than only the first read.
func TestReplaceSlideTextSumsReplies(t *testing.T) {
	svc := newDocsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		req := &slides.BatchUpdatePresentationRequest{}
		decodeInto(t, r, req)
		if len(req.Requests) != 1 {
			t.Errorf("batchUpdate sent %d requests, want 1", len(req.Requests))
		}
		writeJSON(w, slides.BatchUpdatePresentationResponse{
			Replies: []*slides.Response{
				{ReplaceAllText: &slides.ReplaceAllTextResponse{OccurrencesChanged: 2}},
				{ReplaceAllText: &slides.ReplaceAllTextResponse{OccurrencesChanged: 1}},
			},
		})
	})

	count, err := svc.ReplaceSlideText(t.Context(), "p-1", "x", "y", false)
	if err != nil {
		t.Fatalf("ReplaceSlideText: %v", err)
	}
	if count != 3 {
		t.Errorf("occurrences changed = %d, want 3 (2 + 1 summed across replies)", count)
	}
}

// TestListSlideLayouts drives ListSlideLayouts over a fake presentations.get:
// each layout's object ID and display name come back in order, and the
// layout's own name is the display-name fallback when the API omits it.
func TestListSlideLayouts(t *testing.T) {
	svc := newDocsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/v1/presentations/p-1" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		writeJSON(w, slides.Presentation{
			PresentationId: "p-1",
			Layouts: []*slides.Page{
				{ObjectId: "layout_title_only", LayoutProperties: &slides.LayoutProperties{DisplayName: "TITLE_ONLY"}},
				{ObjectId: "layout_named", LayoutProperties: &slides.LayoutProperties{Name: "MASTER_TITLE"}},
				{ObjectId: "layout_bare"},
			},
		})
	})

	layouts, err := svc.ListSlideLayouts(t.Context(), "p-1")
	if err != nil {
		t.Fatalf("ListSlideLayouts: %v", err)
	}
	want := []SlideLayout{
		{ObjectID: "layout_title_only", Name: "TITLE_ONLY"},
		{ObjectID: "layout_named", Name: "MASTER_TITLE"},
		{ObjectID: "layout_bare", Name: ""},
	}
	if len(layouts) != len(want) {
		t.Fatalf("layouts = %+v, want %d", layouts, len(want))
	}
	for i, w := range want {
		if layouts[i] != w {
			t.Errorf("layouts[%d] = %+v, want %+v", i, layouts[i], w)
		}
	}
}

// TestListSlidePlaceholders drives ListSlidePlaceholders: placeholder shapes
// come back in slide order with their 1-based slide number, slide object ID,
// placeholder index, and shape object ID; non-placeholder shapes and
// non-shape elements are skipped.
func TestListSlidePlaceholders(t *testing.T) {
	svc := newDocsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/v1/presentations/p-1" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		writeJSON(w, slides.Presentation{
			PresentationId: "p-1",
			Slides: []*slides.Page{
				{ObjectId: "slide_1", PageElements: []*slides.PageElement{
					{ObjectId: "shape_title_1", Shape: &slides.Shape{Placeholder: &slides.Placeholder{Index: 0}}},
					{ObjectId: "textbox", Shape: &slides.Shape{}},
					{ObjectId: "image", Image: &slides.Image{}},
				}},
				{ObjectId: "slide_2", PageElements: []*slides.PageElement{
					{ObjectId: "shape_body_2", Shape: &slides.Shape{Placeholder: &slides.Placeholder{Index: 1}}},
				}},
			},
		})
	})

	placeholders, err := svc.ListSlidePlaceholders(t.Context(), "p-1")
	if err != nil {
		t.Fatalf("ListSlidePlaceholders: %v", err)
	}
	want := []SlidePlaceholder{
		{SlideNumber: 1, SlideID: "slide_1", PlaceholderIndex: 0, ShapeID: "shape_title_1"},
		{SlideNumber: 2, SlideID: "slide_2", PlaceholderIndex: 1, ShapeID: "shape_body_2"},
	}
	if len(placeholders) != len(want) {
		t.Fatalf("placeholders = %+v, want %d", placeholders, len(want))
	}
	for i, w := range want {
		if placeholders[i] != w {
			t.Errorf("placeholders[%d] = %+v, want %+v", i, placeholders[i], w)
		}
	}
}

// TestCreateSlideSendsLayoutID drives CreateSlide over a fake batchUpdate:
// exactly one CreateSlideRequest carries the layout object ID and no
// insertion index when index is nil, and the reply's object ID comes back.
func TestCreateSlideSendsLayoutID(t *testing.T) {
	var got *slides.BatchUpdatePresentationRequest
	var raw string
	svc := newDocsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/v1/presentations/p-1:batchUpdate" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		body := readBody(t, r)
		raw = body
		req := &slides.BatchUpdatePresentationRequest{}
		if err := json.Unmarshal([]byte(body), req); err != nil {
			t.Errorf("decoding request body: %v", err)
		}
		got = req
		writeJSON(w, slides.BatchUpdatePresentationResponse{
			Replies: []*slides.Response{{CreateSlide: &slides.CreateSlideResponse{ObjectId: "new_slide"}}},
		})
	})

	id, err := svc.CreateSlide(t.Context(), "p-1", "layout_title_only", nil)
	if err != nil {
		t.Fatalf("CreateSlide: %v", err)
	}
	if id != "new_slide" {
		t.Errorf("CreateSlide id = %q, want new_slide", id)
	}
	if len(got.Requests) != 1 || got.Requests[0].CreateSlide == nil {
		t.Fatalf("request kind = %+v, want one createSlide", got.Requests[0])
	}
	create := got.Requests[0].CreateSlide
	if create.SlideLayoutReference == nil || create.SlideLayoutReference.LayoutId != "layout_title_only" {
		t.Fatalf("slideLayoutReference = %+v, want layoutId layout_title_only", create.SlideLayoutReference)
	}
	if strings.Contains(raw, "insertionIndex") {
		t.Errorf("request body %s carries an insertionIndex, want none when index is nil", raw)
	}
}

// TestCreateSlideForcesIndexZero proves --index 0 survives the wire: the
// zero-valued field is omitted by default, so the service must force-send it.
func TestCreateSlideForcesIndexZero(t *testing.T) {
	var raw string
	svc := newDocsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		raw = readBody(t, r)
		writeJSON(w, slides.BatchUpdatePresentationResponse{
			Replies: []*slides.Response{{CreateSlide: &slides.CreateSlideResponse{ObjectId: "new_slide"}}},
		})
	})

	index := int64(0)
	if _, err := svc.CreateSlide(t.Context(), "p-1", "layout_title_only", &index); err != nil {
		t.Fatalf("CreateSlide: %v", err)
	}
	if !strings.Contains(raw, `"insertionIndex":0`) {
		t.Errorf("request body %s missing insertionIndex 0 (force-sent)", raw)
	}
}

// TestCreateSlideErrorsWithoutReplyID guards the reply parse: a reply without
// a slide ID must error, not return an empty ID.
func TestCreateSlideErrorsWithoutReplyID(t *testing.T) {
	svc := newDocsTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, slides.BatchUpdatePresentationResponse{
			Replies: []*slides.Response{{CreateSlide: &slides.CreateSlideResponse{}}},
		})
	})

	if _, err := svc.CreateSlide(t.Context(), "p-1", "layout_title_only", nil); err == nil {
		t.Fatal("CreateSlide: want error when the reply carries no slide ID, got nil")
	}
}

// TestInsertSlideTextSendsShapeAndIndex drives InsertSlideText over a fake
// batchUpdate: exactly one InsertTextRequest targets the given shape at
// insertion index 0 with the caller's text.
func TestInsertSlideTextSendsShapeAndIndex(t *testing.T) {
	var got *slides.BatchUpdatePresentationRequest
	svc := newDocsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/v1/presentations/p-1:batchUpdate" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		req := &slides.BatchUpdatePresentationRequest{}
		decodeInto(t, r, req)
		got = req
		writeJSON(w, &slides.BatchUpdatePresentationResponse{})
	})

	if err := svc.InsertSlideText(t.Context(), "p-1", "shape_title_2", "Hello"); err != nil {
		t.Fatalf("InsertSlideText: %v", err)
	}
	if len(got.Requests) != 1 || got.Requests[0].InsertText == nil {
		t.Fatalf("request kind = %+v, want one insertText", got.Requests[0])
	}
	ins := got.Requests[0].InsertText
	if ins.ObjectId != "shape_title_2" {
		t.Errorf("objectId = %q, want shape_title_2", ins.ObjectId)
	}
	if ins.InsertionIndex != 0 {
		t.Errorf("insertionIndex = %d, want 0 (the start of the shape)", ins.InsertionIndex)
	}
	if ins.Text != "Hello" {
		t.Errorf("text = %q, want Hello", ins.Text)
	}
}

// TestInsertSlideTextPropagatesAPIError guards the error path: an API failure
// must surface wrapped, naming the presentation.
func TestInsertSlideTextPropagatesAPIError(t *testing.T) {
	svc := newDocsTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error": {"code": 403, "message": "no access"}}`, http.StatusForbidden)
	})

	if err := svc.InsertSlideText(t.Context(), "p-1", "shape", "x"); err == nil {
		t.Fatal("InsertSlideText: want error on API failure, got nil")
	}
}
