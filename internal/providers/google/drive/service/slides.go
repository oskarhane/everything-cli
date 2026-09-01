package service

import (
	"context"
	"fmt"
	"strings"

	slides "google.golang.org/api/slides/v1"
)

// SlideShape is one shape's text on one slide: SlideNumber is the 1-based
// slide position, ShapeID the shape's (page element's) object ID, and Text
// the shape's text runs joined in order.
type SlideShape struct {
	SlideNumber int
	ShapeID     string
	Text        string
}

// SlideService is the Slides API surface the slides leaves use: reading a
// presentation's shape text and replacing text across all slides.
type SlideService interface {
	GetSlideText(ctx context.Context, id string) ([]SlideShape, error)
	ReplaceSlideText(ctx context.Context, id, find, replaceWith string, matchCase bool) (int, error)
}

// GetSlideText returns one SlideShape per slide shape that has non-empty
// text. It reads the whole presentation (no field mask: text lives three
// levels down and a too-narrow mask drops shapes entirely). Text is joined
// verbatim — control-byte stripping is the output layer's job, not the
// seam's.
func (s *realDriveService) GetSlideText(ctx context.Context, id string) ([]SlideShape, error) {
	pres, err := s.slides.Presentations.Get(id).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("getting presentation %s: %w", id, err)
	}
	var shapes []SlideShape
	for i, slide := range pres.Slides {
		if slide == nil {
			continue
		}
		for _, element := range slide.PageElements {
			text := slideShapeText(element)
			if text == "" {
				continue
			}
			shapes = append(shapes, SlideShape{
				SlideNumber: i + 1,
				ShapeID:     element.ObjectId,
				Text:        text,
			})
		}
	}
	return shapes, nil
}

// slideShapeText joins a page element's shape text runs in order, returning
// "" for anything without shape text (images, tables, groups, ...).
func slideShapeText(element *slides.PageElement) string {
	if element == nil || element.Shape == nil || element.Shape.Text == nil {
		return ""
	}
	var runs []string
	for _, te := range element.Shape.Text.TextElements {
		if te != nil && te.TextRun != nil {
			runs = append(runs, te.TextRun.Content)
		}
	}
	return strings.Join(runs, "")
}

// ReplaceSlideText replaces every occurrence of find (case-insensitive unless
// matchCase) across every slide in ONE batchUpdate and returns the total
// number of occurrences changed (the API reports per-request, so the count
// arrives as the single reply's value).
func (s *realDriveService) ReplaceSlideText(ctx context.Context, id, find, replaceWith string, matchCase bool) (int, error) {
	resp, err := s.slides.Presentations.BatchUpdate(id, &slides.BatchUpdatePresentationRequest{
		Requests: []*slides.Request{{
			ReplaceAllText: &slides.ReplaceAllTextRequest{
				ContainsText: &slides.SubstringMatchCriteria{Text: find, MatchCase: matchCase},
				ReplaceText:  replaceWith,
			},
		}},
	}).Context(ctx).Do()
	if err != nil {
		return 0, fmt.Errorf("replacing text in presentation %s: %w", id, err)
	}
	var count int64
	for _, reply := range resp.Replies {
		if reply != nil && reply.ReplaceAllText != nil {
			count += reply.ReplaceAllText.OccurrencesChanged
		}
	}
	return int(count), nil
}
