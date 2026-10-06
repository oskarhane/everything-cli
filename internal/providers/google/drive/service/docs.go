package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"
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
// ResolveDocTab. InsertDocTable resolves its tab key like AppendDocText and
// optionally fills the new table's cells — spec.Index must be >= 0: only 0
// computes the tab's end-of-body index, a negative index is an error.
// FormatDocRange styles a range in one tab and, like InsertDocText,
// forwards the tab ID as-is — at least one style bool or a heading level
// must be set: an empty format errors before any batchUpdate is issued.
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
	InsertDocTable(ctx context.Context, docID string, spec DocTableSpec) (int64, error)
	FormatDocRange(ctx context.Context, docID string, format DocRangeFormat) error
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

// DocTableSpec describes an InsertDocTable call: Rows and Columns give the
// grid shape; Index is the Docs-API content index the table inserts before
// (0 computes the chosen tab's end-of-body index, the same endBodyIndex
// rule AppendDocText uses; a negative index is an error); TabKey resolves
// the target tab (exact tab ID first, then exact title; "" = first tab);
// Cells carries row-major cell texts — empty or nil leaves the table empty
// and skips the fill pass.
type DocTableSpec struct {
	Rows    int64
	Columns int64
	Index   int64
	TabKey  string
	Cells   [][]string
}

// InsertDocTable inserts a spec.Rows x spec.Columns table into a tab in ONE
// batchUpdate and returns the table's start index: the insertion index + 1,
// since the API inserts a newline ahead of the table itself. Only
// spec.Index == 0 computes the tab's end-of-body index; a negative index
// errors before any API call. When spec.Cells is non-empty, a fill pass
// re-reads the (updated) tabs tree, finds the inserted table's structural
// element at that start index, and writes the cell texts. The tab key is
// resolved against the tabs tree like AppendDocText does, so a title key
// pins the real tab ID on the wire.
func (s *realDriveService) InsertDocTable(ctx context.Context, docID string, spec DocTableSpec) (int64, error) {
	if spec.Index < 0 {
		return 0, fmt.Errorf("inserting table into document %s: index %d is negative: 0 inserts at the end of the tab body", docID, spec.Index)
	}
	doc, err := s.getDocumentTabs(ctx, docID)
	if err != nil {
		return 0, err
	}
	tab, err := chooseTab(doc, spec.TabKey)
	if err != nil {
		return 0, fmt.Errorf("choosing tab in document %s: %w", docID, err)
	}
	index := spec.Index
	if index == 0 {
		body, err := tabBody(tab)
		if err != nil {
			return 0, fmt.Errorf("computing table index for document %s: %w", docID, err)
		}
		index, err = endBodyIndex(body)
		if err != nil {
			return 0, fmt.Errorf("computing table index for document %s: %w", docID, err)
		}
	}
	if _, err := s.docs.Documents.BatchUpdate(docID, &docs.BatchUpdateDocumentRequest{
		Requests: []*docs.Request{{
			InsertTable: &docs.InsertTableRequest{
				Rows:     spec.Rows,
				Columns:  spec.Columns,
				Location: tabLocation(index, spec.TabKey, tab),
			},
		}},
	}).Context(ctx).Do(); err != nil {
		return 0, fmt.Errorf("inserting table into document %s: %w", docID, err)
	}
	start := index + 1
	if len(spec.Cells) == 0 {
		return start, nil
	}
	if err := s.fillDocTableCells(ctx, docID, spec, start); err != nil {
		return 0, err
	}
	return start, nil
}

// fillDocTableCells writes spec.Cells into the table whose structural
// element starts at start. The insert shifted every index after it, so the
// tabs tree is re-read and the tab re-resolved before locating the table.
// The texts go out in ONE batchUpdate of InsertTextRequests, one per
// non-empty cell at the cell's first-paragraph start index, ordered by
// DESCENDING index so an earlier insert cannot shift a later one's index.
func (s *realDriveService) fillDocTableCells(ctx context.Context, docID string, spec DocTableSpec, start int64) error {
	doc, err := s.getDocumentTabs(ctx, docID)
	if err != nil {
		return err
	}
	tab, err := chooseTab(doc, spec.TabKey)
	if err != nil {
		return fmt.Errorf("choosing tab in document %s: %w", docID, err)
	}
	body, err := tabBody(tab)
	if err != nil {
		return fmt.Errorf("filling table in document %s: %w", docID, err)
	}
	table := tableElementAt(body, start)
	if table == nil {
		return fmt.Errorf("filling table in document %s: no table element at index %d", docID, start)
	}
	type cellInsert struct {
		index int64
		text  string
	}
	var inserts []cellInsert
	for r, row := range table.TableRows {
		if row == nil {
			continue
		}
		for c, cell := range row.TableCells {
			text := specCellText(spec.Cells, r, c)
			if text == "" || cell == nil || len(cell.Content) == 0 || cell.Content[0] == nil {
				continue
			}
			inserts = append(inserts, cellInsert{index: cell.Content[0].StartIndex, text: text})
		}
	}
	sort.Slice(inserts, func(i, j int) bool { return inserts[i].index > inserts[j].index })
	if len(inserts) == 0 {
		return nil
	}
	requests := make([]*docs.Request, 0, len(inserts))
	for _, ins := range inserts {
		requests = append(requests, &docs.Request{
			InsertText: &docs.InsertTextRequest{
				Location: tabLocation(ins.index, spec.TabKey, tab),
				Text:     ins.text,
			},
		})
	}
	if _, err := s.docs.Documents.BatchUpdate(docID, &docs.BatchUpdateDocumentRequest{
		Requests: requests,
	}).Context(ctx).Do(); err != nil {
		return fmt.Errorf("filling table cells in document %s: %w", docID, err)
	}
	return nil
}

// tableElementAt returns the table of the body structural element starting
// at index start, or nil when no such element exists or it is not a table.
func tableElementAt(body *docs.Body, start int64) *docs.Table {
	if body == nil {
		return nil
	}
	for _, el := range body.Content {
		if el != nil && el.StartIndex == start && el.Table != nil {
			return el.Table
		}
	}
	return nil
}

// specCellText returns the text for the cell at row r, column c: spec rows
// shorter than the column count (or missing entirely) read as empty, which
// the fill pass skips.
func specCellText(cells [][]string, r, c int) string {
	if r >= len(cells) || c >= len(cells[r]) {
		return ""
	}
	return cells[r][c]
}

// DocRangeFormat describes a FormatDocRange call: StartIndex and EndIndex
// bound the range (Docs-API content indexes, zero-based UTF-16 code units);
// TabID pins the range to a tab ("" = the API's first-tab default); Bold,
// Italic, Strikethrough, and Underline switch those text styles on;
// HeadingLevel > 0 restyles the covered paragraphs as HEADING_<n>. At least
// one style bool or a heading level must be set: an empty format is an
// error, not an empty batchUpdate.
type DocRangeFormat struct {
	StartIndex    int64
	EndIndex      int64
	TabID         string
	Bold          bool
	Italic        bool
	Strikethrough bool
	Underline     bool
	HeadingLevel  int64
}

// FormatDocRange styles a range in ONE batchUpdate: one
// UpdateTextStyleRequest whose TextStyle carries only the requested style
// bools true and whose fields mask names exactly those fields (so
// unrequested styles stay untouched), plus — when HeadingLevel is set — one
// UpdateParagraphStyleRequest naming HEADING_<n>. With no style bool set
// and HeadingLevel 0 there is nothing to send, so the call errors before
// any batchUpdate is issued. The tab ID is forwarded as-is: like
// InsertDocText, this call makes no read, so a title key must be resolved
// first via ResolveDocTab.
func (s *realDriveService) FormatDocRange(ctx context.Context, docID string, format DocRangeFormat) error {
	if !format.Bold && !format.Italic && !format.Strikethrough && !format.Underline && format.HeadingLevel <= 0 {
		return fmt.Errorf("formatting range in document %s: no style requested: set at least one style bool or a heading level", docID)
	}
	rng := &docs.Range{StartIndex: format.StartIndex, EndIndex: format.EndIndex, TabId: format.TabID}
	var requests []*docs.Request
	style, fields := textStyleOf(format)
	if fields != "" {
		requests = append(requests, &docs.Request{
			UpdateTextStyle: &docs.UpdateTextStyleRequest{
				Range:     rng,
				TextStyle: style,
				Fields:    fields,
			},
		})
	}
	if format.HeadingLevel > 0 {
		requests = append(requests, &docs.Request{
			UpdateParagraphStyle: &docs.UpdateParagraphStyleRequest{
				Range:          rng,
				ParagraphStyle: &docs.ParagraphStyle{NamedStyleType: fmt.Sprintf("HEADING_%d", format.HeadingLevel)},
				Fields:         "namedStyleType",
			},
		})
	}
	if _, err := s.docs.Documents.BatchUpdate(docID, &docs.BatchUpdateDocumentRequest{
		Requests: requests,
	}).Context(ctx).Do(); err != nil {
		return fmt.Errorf("formatting range in document %s: %w", docID, err)
	}
	return nil
}

// textStyleOf builds the TextStyle carrying exactly the requested style
// bools plus the fields mask naming them ("bold,italic,..."); an empty mask
// means no text style was requested and the caller omits the request.
func textStyleOf(f DocRangeFormat) (*docs.TextStyle, string) {
	style := &docs.TextStyle{
		Bold:          f.Bold,
		Italic:        f.Italic,
		Strikethrough: f.Strikethrough,
		Underline:     f.Underline,
	}
	var fields []string
	if f.Bold {
		fields = append(fields, "bold")
	}
	if f.Italic {
		fields = append(fields, "italic")
	}
	if f.Strikethrough {
		fields = append(fields, "strikethrough")
	}
	if f.Underline {
		fields = append(fields, "underline")
	}
	return style, strings.Join(fields, ",")
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
