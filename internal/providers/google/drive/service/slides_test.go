package service

import (
	"net/http"
	"testing"

	slides "google.golang.org/api/slides/v1"
)

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
