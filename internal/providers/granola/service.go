package granola

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// defaultBaseURL is the Granola public API origin; paths are /v1/....
const defaultBaseURL = "https://public-api.granola.ai"

// pageSize is the API's per-request maximum (page_size accepts 1-30, default
// 10); list always asks for full pages.
const pageSize = 30

// maxListPages caps how many pages one listing may follow before giving up.
// A well-behaved server ends with hasMore == false, so the cap only ever
// fires on a misbehaving endpoint looping cursors forever. 100 pages is
// 3000 notes — far beyond any real listing.
const maxListPages = 100

// NoteService is the seam the note leaves consume: the whole Granola HTTP
// surface this CLI uses. The concrete service owns every HTTP call and the
// cursor pagination, so no leaf duplicates either.
type NoteService interface {
	// ListNotes returns every note matching opts, following the cursor
	// across pages.
	ListNotes(ctx context.Context, opts ListOptions) ([]NoteSummary, error)
	// GetNote returns one note by id; includeTranscript inlines the
	// transcript (the API answers 413 TRANSCRIPT_TOO_LARGE when it does
	// not fit one response).
	GetNote(ctx context.Context, id string, includeTranscript bool) (*Note, error)
}

// Compile-time proof that httpService satisfies the seam.
var _ NoteService = (*httpService)(nil)

// ListOptions maps the GET /v1/notes query params. Empty fields are omitted.
type ListOptions struct {
	CreatedBefore string
	CreatedAfter  string
	UpdatedAfter  string
	FolderID      string
}

// Owner is a note's owner; name may be null, email is always present.
type Owner struct {
	Name  *string `json:"name"`
	Email string  `json:"email"`
}

// NoteSummary is one entry of GET /v1/notes. The JSON tags are the API's
// wire names (snake_case except hasMore/cursor on the envelope).
type NoteSummary struct {
	ID        string    `json:"id"`
	Object    string    `json:"object"`
	Title     *string   `json:"title"`
	Owner     Owner     `json:"owner"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Attendee is a meeting attendee; name may be null.
type Attendee struct {
	Name  *string `json:"name"`
	Email string  `json:"email"`
}

// Invitee is a calendar-event invitee.
type Invitee struct {
	Email string `json:"email"`
}

// CalendarEvent is the meeting a note is attached to.
type CalendarEvent struct {
	EventTitle         *string    `json:"event_title"`
	Invitees           []Invitee  `json:"invitees"`
	Organiser          *string    `json:"organiser"`
	CalendarEventID    *string    `json:"calendar_event_id"`
	ScheduledStartTime *time.Time `json:"scheduled_start_time"`
	ScheduledEndTime   *time.Time `json:"scheduled_end_time"`
}

// Folder is one folder membership entry (ancestors included).
type Folder struct {
	ID             string  `json:"id"`
	Object         string  `json:"object"`
	Name           string  `json:"name"`
	ParentFolderID *string `json:"parent_folder_id"`
}

// Speaker identifies one transcript segment's speaker.
type Speaker struct {
	Source           string  `json:"source"`
	Attribution      *string `json:"attribution,omitempty"`
	DiarizationLabel *string `json:"diarization_label,omitempty"`
	Name             *string `json:"name,omitempty"`
}

// TranscriptSegment is one chunk of an inlined transcript.
type TranscriptSegment struct {
	Speaker   Speaker   `json:"speaker"`
	Text      string    `json:"text"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
}

// Note is the GET /v1/notes/{note_id} response. Transcript is only present
// with include=transcript; private notes only when the key belongs to the
// note's creator.
type Note struct {
	ID                   string              `json:"id"`
	Object               string              `json:"object"`
	Title                *string             `json:"title"`
	Owner                Owner               `json:"owner"`
	CreatedAt            time.Time           `json:"created_at"`
	UpdatedAt            time.Time           `json:"updated_at"`
	WebURL               string              `json:"web_url"`
	CalendarEvent        *CalendarEvent      `json:"calendar_event"`
	Attendees            []Attendee          `json:"attendees"`
	FolderMembership     []Folder            `json:"folder_membership"`
	SummaryText          string              `json:"summary_text"`
	SummaryMarkdown      *string             `json:"summary_markdown"`
	PrivateNotesText     *string             `json:"private_notes_text"`
	PrivateNotesMarkdown *string             `json:"private_notes_markdown"`
	Transcript           []TranscriptSegment `json:"transcript"`
}

// listResponse is the GET /v1/notes envelope.
type listResponse struct {
	Notes   []NoteSummary `json:"notes"`
	HasMore bool          `json:"hasMore"`
	Cursor  *string       `json:"cursor"`
}

// httpService is the production NoteService: plain HTTP over the API-key
// strategy's authenticated client. Decoding is strict (unknown fields fail)
// so an upstream schema change breaks one test loudly instead of silently
// dropping data.
type httpService struct {
	client  *http.Client
	baseURL string
}

// newHTTPService binds the service to an authenticated client and API base
// URL. Tests pass an httptest server's URL and client.
func newHTTPService(client *http.Client, baseURL string) *httpService {
	return &httpService{client: client, baseURL: strings.TrimRight(baseURL, "/")}
}

// ListNotes pages GET /v1/notes until the listing is exhausted.
func (s *httpService) ListNotes(ctx context.Context, opts ListOptions) ([]NoteSummary, error) {
	var notes []NoteSummary
	cursor := ""
	for page := 0; ; page++ {
		resp, err := s.listNotesPage(ctx, opts, cursor)
		if err != nil {
			return nil, err
		}
		notes = append(notes, resp.Notes...)
		if !resp.HasMore || resp.Cursor == nil || *resp.Cursor == "" {
			return notes, nil
		}
		if page+1 >= maxListPages {
			return nil, fmt.Errorf("note listing did not terminate after %d pages", maxListPages)
		}
		cursor = *resp.Cursor
	}
}

// listNotesPage fetches one page; cursor is "" for the first page.
func (s *httpService) listNotesPage(ctx context.Context, opts ListOptions, cursor string) (*listResponse, error) {
	q := url.Values{}
	q.Set("page_size", strconv.Itoa(pageSize))
	if opts.CreatedBefore != "" {
		q.Set("created_before", opts.CreatedBefore)
	}
	if opts.CreatedAfter != "" {
		q.Set("created_after", opts.CreatedAfter)
	}
	if opts.UpdatedAfter != "" {
		q.Set("updated_after", opts.UpdatedAfter)
	}
	if opts.FolderID != "" {
		q.Set("folder_id", opts.FolderID)
	}
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	var out listResponse
	if err := s.get(ctx, "/v1/notes", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetNote fetches one note; includeTranscript adds include=transcript.
func (s *httpService) GetNote(ctx context.Context, id string, includeTranscript bool) (*Note, error) {
	q := url.Values{}
	if includeTranscript {
		q.Set("include", "transcript")
	}
	var out Note
	if err := s.get(ctx, "/v1/notes/"+url.PathEscape(id), q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// get issues one authenticated GET and strictly decodes the 200 body into
// out. Non-200 statuses become descriptive errors via statusError.
func (s *httpService) get(ctx context.Context, path string, q url.Values, out any) error {
	u := s.baseURL + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("building granola request: %w", err)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("calling granola API: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return statusError(resp.StatusCode, body)
	}
	dec := json.NewDecoder(resp.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("decoding granola %s response (upstream schema changed?): %w", path, err)
	}
	return nil
}

// statusError renders a non-200 response as an actionable error. The
// well-known statuses carry the API's documented semantics; body detail is
// control-stripped elsewhere by the output layer when it is printed.
func statusError(code int, body []byte) error {
	detail := strings.TrimSpace(string(body))
	switch code {
	case http.StatusUnauthorized:
		return fmt.Errorf("granola API rejected the API key (401): add a valid key with `granola account add`")
	case http.StatusNotFound:
		return fmt.Errorf("granola note not found (404): the note may still be processing or was never summarized")
	case http.StatusRequestEntityTooLarge:
		return fmt.Errorf("granola transcript too large (413 TRANSCRIPT_TOO_LARGE): the API serves oversized transcripts in pages from GET /v1/notes/<note_id>/transcript, which this CLI does not support yet — retry without --include-transcript for the summary")
	case http.StatusTooManyRequests:
		return fmt.Errorf("granola API rate limit exceeded (429): limits are 25 requests per 5s burst and 5 requests/s sustained — retry shortly")
	}
	return fmt.Errorf("granola API returned %d: %s", code, detail)
}
