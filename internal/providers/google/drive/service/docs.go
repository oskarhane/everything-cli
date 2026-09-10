package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	docs "google.golang.org/api/docs/v1"
)

// DocService is the Docs API surface the docs leaves use: thin ctx-first
// wrappers, so fakes model documents, not call objects. GetDocText returns
// the document's plain-text export (text/plain); ListDocTabs flattens the
// document's tab tree depth-first; GetDocTabText renders one tab's body as
// plain text; AddDocTab, DeleteDocTab, and RenameDocTab manage tabs;
// AppendDocText adds text at the very end of a tab's body ("" = first tab);
// InsertDocText inserts text before the given Docs-API content index in a tab
// ("" = first tab); ReplaceDocText replaces every occurrence of find
// (case-sensitive iff matchCase) doc-wide and returns how many occurrences
// changed. AppendDocText resolves its tab key (exact tab ID, then exact
// title) because it reads the tabs tree anyway; InsertDocText makes no read
// and forwards the tab ID as-is, so a title key must be resolved first via
// ResolveDocTab.
type DocService interface {
	GetDocText(ctx context.Context, docID string) (string, error)
	ListDocTabs(ctx context.Context, docID string) ([]DocTab, error)
	ResolveDocTab(ctx context.Context, docID, key string) (DocTab, error)
	GetDocTabText(ctx context.Context, docID, tabKey string) (string, error)
	AddDocTab(ctx context.Context, docID, title string) (string, error)
	DeleteDocTab(ctx context.Context, docID, tabID string) error
	RenameDocTab(ctx context.Context, docID, tabID, title string) error
	AppendDocText(ctx context.Context, docID, text, tabKey string) (err error)
	InsertDocText(ctx context.Context, docID, text string, index int64, tabID string) (err error)
	ReplaceDocText(ctx context.Context, docID, find, replaceWith string, matchCase bool) (int, error)
}

// DocTab is one tab of a document, output-facing (snake_case JSON keys, the
// convention for every field this CLI emits). The list is flattened
// depth-first, so a parent always precedes its child tabs; ParentTabID is
// empty for root-level tabs.
type DocTab struct {
	TabID        string `json:"tab_id"`
	Title        string `json:"title"`
	Index        int64  `json:"index"`
	NestingLevel int64  `json:"nesting_level"`
	ParentTabID  string `json:"parent_tab_id"`
}

// textExportMime is the export mimeType GetDocText reads. Exported content is
// capped at 10 MB by the API, so reading the whole export into memory is
// bounded; the plain-text conversion is not lossy for textual use.
const textExportMime = "text/plain"

// tabsFields is the get mask for tab reads: the whole tabs tree, including
// each tab's documentTab.body. A top-level field name selects all nested
// sub-fields, which matters here — child tabs nest to arbitrary depth, so a
// spelled-out path (tabs.childTabs.tabProperties...) would miss deep levels.
const tabsFields = "tabs"

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

// ListDocTabs returns the document's tabs flattened depth-first (a parent
// before its child tabs, recursively): it gets the document with
// includeTabsContent=true, so Document.tabs — not the legacy top-level body —
// is populated.
func (s *realDriveService) ListDocTabs(ctx context.Context, docID string) ([]DocTab, error) {
	doc, err := s.getDocumentTabs(ctx, docID)
	if err != nil {
		return nil, err
	}
	flat := flattenTabs(doc.Tabs)
	out := make([]DocTab, 0, len(flat))
	for _, tab := range flat {
		out = append(out, docTabOf(tab))
	}
	return out, nil
}

// ResolveDocTab resolves a tab key — exact tab ID first, then exact title —
// against the document's tab tree and returns the matched tab. Write leaves
// whose write call makes no read of its own (InsertDocText) call it so the
// wire always carries the immutable tab ID, never the caller's key.
func (s *realDriveService) ResolveDocTab(ctx context.Context, docID, key string) (DocTab, error) {
	doc, err := s.getDocumentTabs(ctx, docID)
	if err != nil {
		return DocTab{}, err
	}
	tab, err := chooseTab(doc, key)
	if err != nil {
		return DocTab{}, err
	}
	return docTabOf(tab), nil
}

// GetDocTabText renders one tab's body to plain text: the tab is resolved by
// exact tab ID first, then exact title ("" = first tab).
//
// Fidelity caveat: this is a lossy convenience render, not an export —
// it keeps paragraph text runs and table cell text only (one line per
// paragraph; table structure is flattened into the line stream) and drops
// styling, images and other non-text elements, section breaks, and any
// headers, footers, or footnotes.
func (s *realDriveService) GetDocTabText(ctx context.Context, docID, tabKey string) (string, error) {
	doc, err := s.getDocumentTabs(ctx, docID)
	if err != nil {
		return "", err
	}
	tab, err := chooseTab(doc, tabKey)
	if err != nil {
		return "", fmt.Errorf("choosing tab in document %s: %w", docID, err)
	}
	body, err := tabBody(tab)
	if err != nil {
		return "", fmt.Errorf("tab %s in document %s: %w", tabKey, docID, err)
	}
	return renderBodyText(body), nil
}

// AddDocTab adds a tab titled title to the document and returns the new
// tab's ID from the batchUpdate reply. The API reports the created tab's
// properties (which carry the immutable ID) in the reply.
func (s *realDriveService) AddDocTab(ctx context.Context, docID, title string) (string, error) {
	resp, err := s.docs.Documents.BatchUpdate(docID, &docs.BatchUpdateDocumentRequest{
		Requests: []*docs.Request{{
			AddDocumentTab: &docs.AddDocumentTabRequest{
				TabProperties: &docs.TabProperties{Title: title},
			},
		}},
	}).Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("adding tab to document %s: %w", docID, err)
	}
	for _, reply := range resp.Replies {
		if reply == nil || reply.AddDocumentTab == nil || reply.AddDocumentTab.TabProperties == nil {
			continue
		}
		if id := reply.AddDocumentTab.TabProperties.TabId; id != "" {
			return id, nil
		}
	}
	return "", fmt.Errorf("adding tab to document %s: reply carried no tab ID", docID)
}

// DeleteDocTab deletes a tab; the API deletes its child tabs with it.
func (s *realDriveService) DeleteDocTab(ctx context.Context, docID, tabID string) error {
	if _, err := s.docs.Documents.BatchUpdate(docID, &docs.BatchUpdateDocumentRequest{
		Requests: []*docs.Request{{
			DeleteTab: &docs.DeleteTabRequest{TabId: tabID},
		}},
	}).Context(ctx).Do(); err != nil {
		return fmt.Errorf("deleting tab %s from document %s: %w", tabID, docID, err)
	}
	return nil
}

// RenameDocTab renames a tab. The update mask covers "title" only; the root
// tab_properties is implied and must not be listed in the mask.
func (s *realDriveService) RenameDocTab(ctx context.Context, docID, tabID, title string) error {
	if _, err := s.docs.Documents.BatchUpdate(docID, &docs.BatchUpdateDocumentRequest{
		Requests: []*docs.Request{{
			UpdateDocumentTabProperties: &docs.UpdateDocumentTabPropertiesRequest{
				TabProperties: &docs.TabProperties{TabId: tabID, Title: title},
				Fields:        "title",
			},
		}},
	}).Context(ctx).Do(); err != nil {
		return fmt.Errorf("renaming tab %s in document %s: %w", tabID, docID, err)
	}
	return nil
}

// AppendDocText appends text at the very end of a tab's body: it reads the
// tabs tree, computes the index just before the chosen tab's implicit final
// newline (endBodyIndex), then issues ONE batchUpdate carrying a single
// InsertTextRequest. Docs indexes are zero-based UTF-16 code units and an
// insertion point may not be the body's end index, so the last element's
// endIndex - 1 is the only index the API accepts for a true append. An empty
// tabKey targets the first tab; the fetch must use includeTabsContent=true
// because the legacy top-level body is empty for multi-tab documents.
func (s *realDriveService) AppendDocText(ctx context.Context, docID, text, tabKey string) error {
	doc, err := s.getDocumentTabs(ctx, docID)
	if err != nil {
		return err
	}
	tab, err := chooseTab(doc, tabKey)
	if err != nil {
		return fmt.Errorf("choosing tab in document %s: %w", docID, err)
	}
	body, err := tabBody(tab)
	if err != nil {
		return fmt.Errorf("computing append index for document %s: %w", docID, err)
	}
	index, err := endBodyIndex(body)
	if err != nil {
		return fmt.Errorf("computing append index for document %s: %w", docID, err)
	}
	if _, err := s.docs.Documents.BatchUpdate(docID, &docs.BatchUpdateDocumentRequest{
		Requests: []*docs.Request{{
			InsertText: &docs.InsertTextRequest{
				Location: tabLocation(index, tabKey, tab),
				Text:     text,
			},
		}},
	}).Context(ctx).Do(); err != nil {
		return fmt.Errorf("appending text to document %s: %w", docID, err)
	}
	return nil
}

// InsertDocText inserts text immediately before the given Docs-API content
// index (zero-based UTF-16 code units, as the API defines them) in ONE
// batchUpdate carrying a single InsertTextRequest. The index is the caller's:
// the leaf validates it, the service only forwards it. An empty tabID lets
// the API apply the insert to the first tab.
func (s *realDriveService) InsertDocText(ctx context.Context, docID, text string, index int64, tabID string) error {
	if _, err := s.docs.Documents.BatchUpdate(docID, &docs.BatchUpdateDocumentRequest{
		Requests: []*docs.Request{{
			InsertText: &docs.InsertTextRequest{
				// The index is the caller's, and the tab key is forwarded
				// as-is: this call makes no read to resolve a title, so an
				// empty tabID stays omitted (the API's first-tab default).
				Location: &docs.Location{Index: index, TabId: tabID},
				Text:     text,
			},
		}},
	}).Context(ctx).Do(); err != nil {
		return fmt.Errorf("inserting text into document %s: %w", docID, err)
	}
	return nil
}

// tabBody returns the chosen tab's document-tab body, erroring when the API
// response omits it — a tab without a documentTab would otherwise nil-panic
// the caller instead of failing the read or write.
func tabBody(tab *docs.Tab) (*docs.Body, error) {
	if tab == nil || tab.DocumentTab == nil || tab.DocumentTab.Body == nil {
		return nil, errors.New("tab has no readable documentTab body")
	}
	return tab.DocumentTab.Body, nil
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
// occurrences changed. The replace is doc-wide: the API has no tab-scoped
// replaceAllText, so every tab is covered and no tabID exists here.
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

// getDocumentTabs gets the document with includeTabsContent=true so the tabs
// tree (including each tab's documentTab.body) is populated; on this read the
// legacy top-level body is empty, so callers must read tab bodies, not
// doc.Body.
func (s *realDriveService) getDocumentTabs(ctx context.Context, docID string) (*docs.Document, error) {
	doc, err := s.docs.Documents.Get(docID).IncludeTabsContent(true).Fields(tabsFields).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("getting document %s tabs: %w", docID, err)
	}
	return doc, nil
}

// flattenTabs flattens a tab tree depth-first: each tab before its child
// tabs, recursively, so list order matches how a reader scans the document.
func flattenTabs(tabs []*docs.Tab) []*docs.Tab {
	flat := make([]*docs.Tab, 0, len(tabs))
	var walk func([]*docs.Tab)
	walk = func(ts []*docs.Tab) {
		for _, tab := range ts {
			if tab == nil {
				continue
			}
			flat = append(flat, tab)
			walk(tab.ChildTabs)
		}
	}
	walk(tabs)
	return flat
}

// docTabOf converts a wire tab to the output-facing DocTab (nil properties
// yield the zero tab rather than a panic — the list must survive a
// property-less tab the API should not have sent).
func docTabOf(tab *docs.Tab) DocTab {
	if tab.TabProperties == nil {
		return DocTab{}
	}
	props := tab.TabProperties
	return DocTab{
		TabID:        props.TabId,
		Title:        props.Title,
		Index:        props.Index,
		NestingLevel: props.NestingLevel,
		ParentTabID:  props.ParentTabId,
	}
}

// chooseTab resolves the tab a read or write targets: an empty key picks the
// first tab in depth-first order (the API's own default for an omitted
// tabId); otherwise the key is matched against exact tab IDs first, then
// exact titles, so an ID always wins over a same-named title.
func chooseTab(doc *docs.Document, key string) (*docs.Tab, error) {
	flat := flattenTabs(doc.Tabs)
	if key == "" {
		if len(flat) == 0 {
			return nil, errors.New("document has no tabs")
		}
		return flat[0], nil
	}
	for _, tab := range flat {
		if tab.TabProperties != nil && tab.TabProperties.TabId == key {
			return tab, nil
		}
	}
	var matches []*docs.Tab
	for _, tab := range flat {
		if tab.TabProperties != nil && tab.TabProperties.Title == key {
			matches = append(matches, tab)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return nil, fmt.Errorf("no tab with ID or title %q", key)
	default:
		ids := make([]string, 0, len(matches))
		for _, tab := range matches {
			ids = append(ids, tab.TabProperties.TabId)
		}
		return nil, fmt.Errorf("tab title %q is ambiguous: matches tabs %s; use a tab ID", key, strings.Join(ids, ", "))
	}
}

// tabLocation builds an insert location at index, pinned to the resolved tab
// when the caller named one: the API applies an omitted tabId to the first
// tab, which is exactly what the empty-key choice computed the index against.
// The ID comes from the resolved tab's properties, so a title key still pins
// the real tab ID.
func tabLocation(index int64, key string, tab *docs.Tab) *docs.Location {
	loc := &docs.Location{Index: index}
	if key != "" && tab != nil && tab.TabProperties != nil {
		loc.TabId = tab.TabProperties.TabId
	}
	return loc
}

// renderBodyText renders body content to plain text: one line per paragraph
// (its text runs concatenated, the paragraph's own trailing newline
// normalized away) and each table cell's paragraphs rendered the same way.
// See GetDocTabText for what this render drops.
func renderBodyText(body *docs.Body) string {
	if body == nil {
		return ""
	}
	var b strings.Builder
	for _, el := range body.Content {
		renderElementText(el, &b)
	}
	return b.String()
}

// renderElementText renders one structural element into the line stream:
// paragraphs as a line, tables cell by cell (each cell's content rendered
// recursively, so nested tables flatten too).
func renderElementText(el *docs.StructuralElement, b *strings.Builder) {
	switch {
	case el == nil:
		return
	case el.Paragraph != nil:
		var line strings.Builder
		for _, pel := range el.Paragraph.Elements {
			if pel != nil && pel.TextRun != nil {
				line.WriteString(pel.TextRun.Content)
			}
		}
		// The API ships a paragraph's newline inside its final run; the
		// renderer owns the newline, so strip the run's copy before adding
		// our own.
		b.WriteString(strings.TrimSuffix(line.String(), "\n"))
		b.WriteByte('\n')
	case el.Table != nil:
		for _, row := range el.Table.TableRows {
			for _, cell := range row.TableCells {
				for _, cellEl := range cell.Content {
					renderElementText(cellEl, b)
				}
			}
		}
	}
}
