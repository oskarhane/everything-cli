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

	assert.Contains(t, note, "space_membership")
	folders, ok := note["folder_membership"].([]any)
	require.True(t, ok)
	require.Len(t, folders, 1)
	assert.Contains(t, folders[0], "space_id")

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

func TestNoteGetAttachments(t *testing.T) {
	// Manually attached photos surface with their snake_case element keys
	// in JSON and TOON output; table output stays the scalar summary row.
	var log requestLog
	srv := newNotesServer(t, &log)
	_, root, out := newGranolaEnv(t)
	stubNotes(t, newHTTPService(srv.Client(), srv.URL))

	stdout, err := execute(t, root, out, "granola", "note", "get", "not_aaa111bbb222", "--format", "json")
	require.NoError(t, err)

	var note map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &note))
	attachments, ok := note["attachments"].([]any)
	require.True(t, ok)
	require.Len(t, attachments, 1)
	att, ok := attachments[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "att_4y6LduVdwSKC27", att["id"])
	assert.Equal(t, "attachment", att["object"])
	assert.Equal(t, "photo", att["type"])
	assert.Equal(t, "https://files.granola.ai/attachments/att_4y6LduVdwSKC27.jpg", att["url"])
	assert.Equal(t, float64(3024), att["width"])
	assert.Equal(t, float64(4032), att["height"])

	// Uniform elements render as a TOON table: the header carries every
	// snake_case element key, the row the values.
	stdout, err = execute(t, root, out, "granola", "note", "get", "not_aaa111bbb222", "--format", "toon")
	require.NoError(t, err)
	assert.Contains(t, stdout, "attachments[#1]{height,id,object,type,url,width}:")
	assert.Contains(t, stdout,
		`4032,att_4y6LduVdwSKC27,attachment,photo,"https://files.granola.ai/attachments/att_4y6LduVdwSKC27.jpg",3024`)

	stdout, err = execute(t, root, out, "granola", "note", "get", "not_aaa111bbb222", "--format", "table")
	require.NoError(t, err)
	assert.Contains(t, stdout, "NOTE_ID")
	assert.NotContains(t, stdout, "ATTACHMENTS")
	assert.NotContains(t, stdout, "attachments")
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
