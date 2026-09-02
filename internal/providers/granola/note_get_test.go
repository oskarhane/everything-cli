package granola

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNoteGetJSON(t *testing.T) {
	var log requestLog
	srv := newNotesServer(t, &log)
	_, root, out := newGranolaEnv(t)
	stubNotes(t, newHTTPService(srv.Client(), srv.URL))

	stdout, err := execute(t, root, out, "granola", "note", "get", "not_aaa111bbb222", "--format", "json")
	require.NoError(t, err)

	var note map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &note))
	assert.Equal(t, "not_aaa111bbb222", note["note_id"])
	assert.NotContains(t, note, "id")
	assert.Equal(t, "Weekly sync", note["title"])
	assert.Equal(t, "https://notes.granola.ai/t/not_aaa111bbb222", note["web_url"])
	assert.Equal(t, "Discussed sprint progress and blockers.", note["summary_text"])

	owner, ok := note["owner"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "ada@example.com", owner["email"])

	attendees, ok := note["attendees"].([]any)
	require.True(t, ok)
	assert.Len(t, attendees, 2)

	cal, ok := note["calendar_event"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "evt_123", cal["calendar_event_id"])

	queries := log.all()
	require.Len(t, queries, 1)
	assert.Empty(t, queries[0]["include"])
}

func TestNoteGetIncludeTranscriptFlag(t *testing.T) {
	var log requestLog
	srv := newNotesServer(t, &log)
	_, root, out := newGranolaEnv(t)
	stubNotes(t, newHTTPService(srv.Client(), srv.URL))

	_, err := execute(t, root, out,
		"granola", "note", "get", "not_aaa111bbb222", "--include-transcript", "--format", "json")
	require.NoError(t, err)
	queries := log.all()
	require.Len(t, queries, 1)
	assert.Equal(t, []string{"transcript"}, queries[0]["include"])
}

func TestNoteGetTable(t *testing.T) {
	var log requestLog
	srv := newNotesServer(t, &log)
	_, root, out := newGranolaEnv(t)
	stubNotes(t, newHTTPService(srv.Client(), srv.URL))

	stdout, err := execute(t, root, out, "granola", "note", "get", "not_aaa111bbb222", "--format", "table")
	require.NoError(t, err)
	assert.Contains(t, stdout, "NOTE_ID")
	assert.Contains(t, stdout, "WEB_URL")
	assert.Contains(t, stdout, "not_aaa111bbb222")
}

func TestNoteGetToon(t *testing.T) {
	var log requestLog
	srv := newNotesServer(t, &log)
	_, root, out := newGranolaEnv(t)
	stubNotes(t, newHTTPService(srv.Client(), srv.URL))

	stdout, err := execute(t, root, out, "granola", "note", "get", "not_aaa111bbb222", "--format", "toon")
	require.NoError(t, err)
	assert.Contains(t, stdout, "note_id: not_aaa111bbb222")
	assert.Contains(t, stdout, "owner:")
}

func TestNoteGet413Guidance(t *testing.T) {
	// The checkpoint's TRANSCRIPT_TOO_LARGE guidance must surface to the
	// user when --include-transcript overflows one response.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
	}))
	t.Cleanup(srv.Close)
	_, root, out := newGranolaEnv(t)
	stubNotes(t, newHTTPService(srv.Client(), srv.URL))

	_, err := execute(t, root, out,
		"granola", "note", "get", "not_aaa111bbb222", "--include-transcript", "--format", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "TRANSCRIPT_TOO_LARGE")
	assert.Contains(t, err.Error(), "--include-transcript")
}
