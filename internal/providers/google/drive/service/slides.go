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

// SlideLayout is one presentation layout, output-facing: the immutable
// object ID the write verbs target and the human-readable display name the
// add leaf resolves --layout by.
type SlideLayout struct {
	ObjectID string `json:"object_id"`
	Name     string `json:"name"`
}

// SlidePlaceholder identifies one placeholder shape: SlideNumber is the
// slide page's 1-based position, SlideID its object ID, PlaceholderIndex the
// placeholder's index on the slide, and ShapeID the shape's object ID — the
// target of set-text's InsertTextRequest.
type SlidePlaceholder struct {
	SlideNumber      int
	SlideID          string
	PlaceholderIndex int64
	ShapeID          string
}

// SlideService is the Slides API surface the slides leaves use: reading a
// presentation's shape text, listing its layouts and placeholder shapes,
// adding a slide from a layout, writing text into a placeholder shape, and
// replacing text across all slides.
type SlideService interface {
	GetSlideText(ctx context.Context, id string) ([]SlideShape, error)
	ListSlideLayouts(ctx context.Context, id string) ([]SlideLayout, error)
	ListSlidePlaceholders(ctx context.Context, id string) ([]SlidePlaceholder, error)
	CreateSlide(ctx context.Context, id, layoutID string, index *int64) (string, error)
	InsertSlideText(ctx context.Context, id, shapeID, text string) error
	ReplaceSlideText(ctx context.Context, id, find, replaceWith string, matchCase bool) (int, error)
}

// GetSlideText returns one SlideShape per slide shape that has non-empty
// text. It reads the whole presentation (no field mask: text lives three
// levels down and a too-narrow mask drops shapes entirely). Text is joined
// verbatim — control-byte stripping is the output layer's job, not the
// seam's.
func (s *realDriveService) GetSlideText(ctx context.Context, id string) ([]SlideShape, error) {
	pres, err := s.getPresentation(ctx, id)
	if err != nil {
		return nil, err
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

// getPresentation reads the whole presentation (no field mask: text and
// placeholder data live three levels down and a too-narrow mask drops shapes
// entirely). It is the shared read behind GetSlideText, ListSlideLayouts,
// and ListSlidePlaceholders.
func (s *realDriveService) getPresentation(ctx context.Context, id string) (*slides.Presentation, error) {
	pres, err := s.slides.Presentations.Get(id).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("getting presentation %s: %w", id, err)
	}
	return pres, nil
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

// ListSlideLayouts returns one SlideLayout per presentation layout: the
// object ID and the display name (falling back to the layout's own name when
// the API omits the display name). Layout order is the API's order, which
// matches the editor's layout picker.
func (s *realDriveService) ListSlideLayouts(ctx context.Context, id string) ([]SlideLayout, error) {
	pres, err := s.getPresentation(ctx, id)
	if err != nil {
		return nil, err
	}
	layouts := make([]SlideLayout, 0, len(pres.Layouts))
	for _, layout := range pres.Layouts {
		if layout == nil {
			continue
		}
		layouts = append(layouts, SlideLayout{ObjectID: layout.ObjectId, Name: slideLayoutName(layout)})
	}
	return layouts, nil
}

// slideLayoutName returns the layout's human-readable name: the display name
// the picker shows, or the layout's own name when the API omits it.
func slideLayoutName(layout *slides.Page) string {
	if layout == nil || layout.LayoutProperties == nil {
		return ""
	}
	if name := layout.LayoutProperties.DisplayName; name != "" {
		return name
	}
	return layout.LayoutProperties.Name
}

// ListSlidePlaceholders returns one SlidePlaceholder per placeholder shape,
// in slide order. Only shapes whose Shape.Placeholder is set are returned:
// text boxes and other non-placeholder shapes are not addressable by
// --placeholder-idx. The whole presentation is read (no field mask) because
// the placeholder data lives three levels down.
func (s *realDriveService) ListSlidePlaceholders(ctx context.Context, id string) ([]SlidePlaceholder, error) {
	pres, err := s.getPresentation(ctx, id)
	if err != nil {
		return nil, err
	}
	var placeholders []SlidePlaceholder
	for i, slide := range pres.Slides {
		if slide == nil {
			continue
		}
		for _, element := range slide.PageElements {
			if element == nil || element.Shape == nil || element.Shape.Placeholder == nil {
				continue
			}
			placeholders = append(placeholders, SlidePlaceholder{
				SlideNumber:      i + 1,
				SlideID:          slide.ObjectId,
				PlaceholderIndex: element.Shape.Placeholder.Index,
				ShapeID:          element.ObjectId,
			})
		}
	}
	return placeholders, nil
}

// CreateSlide adds a slide built from the layout with object ID layoutID in
// ONE batchUpdate and returns the new slide's object ID from the reply. index
// is the optional zero-based insertion position: nil lets the API append the
// slide at the end, while a non-nil pointer — including a pointer to 0, which
// is a valid position — sets InsertionIndex explicitly.
func (s *realDriveService) CreateSlide(ctx context.Context, id, layoutID string, index *int64) (string, error) {
	create := &slides.CreateSlideRequest{
		SlideLayoutReference: &slides.LayoutReference{LayoutId: layoutID},
	}
	if index != nil {
		create.InsertionIndex = *index
		// The wire omits zero-valued fields, so index 0 (a valid front
		// insertion) needs an explicit ForceSendFields entry to survive.
		create.ForceSendFields = append(create.ForceSendFields, "InsertionIndex")
	}
	resp, err := s.slides.Presentations.BatchUpdate(id, &slides.BatchUpdatePresentationRequest{
		Requests: []*slides.Request{{CreateSlide: create}},
	}).Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("adding slide to presentation %s: %w", id, err)
	}
	for _, reply := range resp.Replies {
		if reply == nil || reply.CreateSlide == nil {
			continue
		}
		if objectID := reply.CreateSlide.ObjectId; objectID != "" {
			return objectID, nil
		}
	}
	return "", fmt.Errorf("adding slide to presentation %s: reply carried no slide ID", id)
}

// InsertSlideText writes text into the shape with object ID shapeID in ONE
// batchUpdate carrying a single InsertTextRequest at insertion index 0 — the
// very start of the shape, which is the right spot for the empty placeholders
// a freshly created slide carries (the API inserts at an index; it does not
// overwrite existing text).
func (s *realDriveService) InsertSlideText(ctx context.Context, id, shapeID, text string) error {
	if _, err := s.slides.Presentations.BatchUpdate(id, &slides.BatchUpdatePresentationRequest{
		Requests: []*slides.Request{{
			InsertText: &slides.InsertTextRequest{
				ObjectId:       shapeID,
				InsertionIndex: 0,
				Text:           text,
			},
		}},
	}).Context(ctx).Do(); err != nil {
		return fmt.Errorf("setting text in presentation %s: %w", id, err)
	}
	return nil
}
