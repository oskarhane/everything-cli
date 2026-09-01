package granola

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixture loads a recorded JSON response from testdata. The tests are
// hermetic: fixtures are served by httptest, never the network.
func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	require.NoError(t, err)
	return data
}

// requestLog records the query params of every request the service makes.
type requestLog struct {
	mu      sync.Mutex
	queries []map[string][]string
}

func (l *requestLog) add(r *http.Request) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.queries = append(l.queries, map[string][]string(r.URL.Query()))
}

func (l *requestLog) all() []map[string][]string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]map[string][]string{}, l.queries...)
}

// newNotesServer serves the two recorded list pages keyed by the cursor
// param and the recorded note for any /v1/notes/<id> path.
func newNotesServer(t *testing.T, log *requestLog) *httptest.Server {
	t.Helper()
	page1 := fixture(t, "notes_page1.json")
	page2 := fixture(t, "notes_page2.json")
	noteGet := fixture(t, "note_get.json")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.add(r)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/v1/notes" && r.URL.Query().Get("cursor") == "":
			_, _ = w.Write(page1)
		case r.URL.Path == "/v1/notes" && r.URL.Query().Get("cursor") == "cursor-page-2":
			_, _ = w.Write(page2)
		case r.URL.Path == "/v1/notes/not_aaa111bbb222":
			_, _ = w.Write(noteGet)
		default:
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestListNotesPaginatesAcrossTwoPages(t *testing.T) {
	var log requestLog
	srv := newNotesServer(t, &log)
	svc := newHTTPService(srv.Client(), srv.URL)

	notes, err := svc.ListNotes(context.Background(), ListOptions{
		CreatedAfter: "2026-08-01",
		FolderID:     "fol_abc123def456",
	})
	require.NoError(t, err)
	require.Len(t, notes, 3)
	assert.Equal(t, "not_aaa111bbb222", notes[0].ID)
	assert.Equal(t, "not_ccc333ddd444", notes[1].ID)
	assert.Nil(t, notes[1].Title)
	assert.Equal(t, "grace@example.com", notes[1].Owner.Email)
	assert.Equal(t, "not_eee555fff666", notes[2].ID)

	queries := log.all()
	require.Len(t, queries, 2)
	// Page one carries the filters and the full page size; page two follows
	// the cursor.
	assert.Equal(t, []string{"30"}, queries[0]["page_size"])
	assert.Equal(t, []string{"2026-08-01"}, queries[0]["created_after"])
	assert.Equal(t, []string{"fol_abc123def456"}, queries[0]["folder_id"])
	assert.Empty(t, queries[0]["cursor"])
	assert.Equal(t, []string{"cursor-page-2"}, queries[1]["cursor"])
}

func TestListNotesStrictDecodeFailsLoudly(t *testing.T) {
	// A fixture whose schema drifts from the decoder's expectations (an
	// unknown upstream field) must fail loudly, not silently drop data.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture(t, "notes_bad_schema.json"))
	}))
	t.Cleanup(srv.Close)
	svc := newHTTPService(srv.Client(), srv.URL)

	_, err := svc.ListNotes(context.Background(), ListOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected_upstream_field")
}

func TestGetNoteParsesFullNote(t *testing.T) {
	var log requestLog
	srv := newNotesServer(t, &log)
	svc := newHTTPService(srv.Client(), srv.URL)

	note, err := svc.GetNote(context.Background(), "not_aaa111bbb222", false)
	require.NoError(t, err)
	assert.Equal(t, "not_aaa111bbb222", note.ID)
	assert.Equal(t, "Weekly sync", *note.Title)
	assert.Equal(t, "ada@example.com", note.Owner.Email)
	assert.Equal(t, "https://notes.granola.ai/t/not_aaa111bbb222", note.WebURL)
	assert.Equal(t, "Discussed sprint progress and blockers.", note.SummaryText)
	require.NotNil(t, note.SummaryMarkdown)
	require.NotNil(t, note.CalendarEvent)
	assert.Equal(t, "evt_123", *note.CalendarEvent.CalendarEventID)
	require.Len(t, note.Attendees, 2)
	require.Len(t, note.FolderMembership, 1)
	assert.Nil(t, note.Transcript)

	queries := log.all()
	require.Len(t, queries, 1)
	assert.Empty(t, queries[0]["include"])
}

func TestGetNoteIncludeTranscript(t *testing.T) {
	var log requestLog
	srv := newNotesServer(t, &log)
	svc := newHTTPService(srv.Client(), srv.URL)

	_, err := svc.GetNote(context.Background(), "not_aaa111bbb222", true)
	require.NoError(t, err)
	queries := log.all()
	require.Len(t, queries, 1)
	assert.Equal(t, []string{"transcript"}, queries[0]["include"])
}

func TestStatusErrors(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		wantErr string
	}{
		{"unauthorized", http.StatusUnauthorized, "401"},
		{"not found", http.StatusNotFound, "404"},
		{"transcript too large", http.StatusRequestEntityTooLarge, "TRANSCRIPT_TOO_LARGE"},
		{"rate limited", http.StatusTooManyRequests, "429"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, `{"error":"x"}`, tc.status)
			}))
			t.Cleanup(srv.Close)
			svc := newHTTPService(srv.Client(), srv.URL)

			_, err := svc.GetNote(context.Background(), "not_aaa111bbb222", true)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}
