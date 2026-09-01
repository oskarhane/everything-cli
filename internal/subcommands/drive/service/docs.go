package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	docs "google.golang.org/api/docs/v1"
)

// DocService is the Docs API surface the docs leaves use: thin ctx-first
// wrappers, so fakes model documents, not call objects. GetDocText returns
// the document's plain-text export (text/plain); AppendDocText adds text at
// the very end of the body; ReplaceDocText replaces every occurrence of find
// (case-sensitive iff matchCase) and returns how many occurrences changed.
type DocService interface {
	GetDocText(ctx context.Context, docID string) (string, error)
	AppendDocText(ctx context.Context, docID, text string) (err error)
	ReplaceDocText(ctx context.Context, docID, find, replaceWith string, matchCase bool) (int, error)
}

// textExportMime is the export mimeType GetDocText reads. Exported content is
// capped at 10 MB by the API, so reading the whole export into memory is
// bounded; the plain-text conversion is not lossy for textual use.
const textExportMime = "text/plain"

// GetDocText returns the document's content as plain text: it streams the
// drive export endpoint into a buffer and reads it whole. It reuses the
// shared ExportTo stream so the download path stays in one place.
func (s *realDriveService) GetDocText(ctx context.Context, docID string) (string, error) {
	var buf bytes.Buffer
	if err := s.ExportTo(ctx, docID, textExportMime, &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// AppendDocText appends text at the very end of the document body: it reads
// the body's structural elements, computes the index just before the final
// implicit newline (endBodyIndex), then issues ONE batchUpdate carrying a
// single InsertTextRequest. Docs indexes are zero-based UTF-16 code units and
// an insertion point may not be the body's end index, so the last element's
// endIndex - 1 is the only index the API accepts for a true append.
func (s *realDriveService) AppendDocText(ctx context.Context, docID, text string) error {
	doc, err := s.docs.Documents.Get(docID).Fields("body.content").Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("getting document %s body: %w", docID, err)
	}
	index, err := endBodyIndex(doc.Body)
	if err != nil {
		return fmt.Errorf("computing append index for document %s: %w", docID, err)
	}
	if _, err := s.docs.Documents.BatchUpdate(docID, &docs.BatchUpdateDocumentRequest{
		Requests: []*docs.Request{{
			InsertText: &docs.InsertTextRequest{
				Location: &docs.Location{Index: index},
				Text:     text,
			},
		}},
	}).Context(ctx).Do(); err != nil {
		return fmt.Errorf("appending text to document %s: %w", docID, err)
	}
	return nil
}

// endBodyIndex returns the insertion index that appends text at the very end
// of the body segment: the last body.content StructuralElement's exclusive
// endIndex minus 1 — the position of the body's implicit final newline, the
// last index the API accepts an insertion at. The element's EndIndex is 0
// when the API omitted it, which is indistinguishable from an empty element,
// so both cases error out rather than insert at a bogus index.
func endBodyIndex(body *docs.Body) (int64, error) {
	if body == nil || len(body.Content) == 0 {
		return 0, errors.New("document body has no content elements")
	}
	last := body.Content[len(body.Content)-1]
	if last.EndIndex <= 0 {
		return 0, errors.New("document body's last element has no endIndex")
	}
	return last.EndIndex - 1, nil
}

// ReplaceDocText replaces every occurrence of find (case-insensitive unless
// matchCase) with replaceWith in ONE batchUpdate and returns the number of
// occurrences changed.
func (s *realDriveService) ReplaceDocText(ctx context.Context, docID, find, replaceWith string, matchCase bool) (int, error) {
	resp, err := s.docs.Documents.BatchUpdate(docID, &docs.BatchUpdateDocumentRequest{
		Requests: []*docs.Request{{
			ReplaceAllText: &docs.ReplaceAllTextRequest{
				ContainsText: &docs.SubstringMatchCriteria{Text: find, MatchCase: matchCase},
				ReplaceText:  replaceWith,
			},
		}},
	}).Context(ctx).Do()
	if err != nil {
		return 0, fmt.Errorf("replacing text in document %s: %w", docID, err)
	}
	var count int64
	for _, reply := range resp.Replies {
		if reply != nil && reply.ReplaceAllText != nil {
			count += reply.ReplaceAllText.OccurrencesChanged
		}
	}
	return int(count), nil
}
