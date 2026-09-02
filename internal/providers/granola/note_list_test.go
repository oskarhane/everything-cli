package granola

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNoteListJSON(t *testing.T) {
	var log requestLog
	srv := newNotesServer(t, &log)
	_, root, out := newGranolaEnv(t)
	stubNotes(t, newHTTPService(srv.Client(), srv.URL))

	stdout, err := execute(t, root, out, "granola", "note", "list", "--format", "json")
	require.NoError(t, err)

	// Both pages followed: three notes in one array with snake_case keys.
	var notes []map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &notes))
	require.Len(t, notes, 3)
	assert.Equal(t, "not_aaa111bbb222", notes[0]["note_id"])
	assert.Equal(t, "Weekly sync", notes[0]["title"])
	assert.Equal(t, "ada@example.com", notes[0]["owner"])
	assert.Equal(t, "2026-08-20T14:00:00Z", notes[0]["created_at"])
	assert.Equal(t, "2026-08-20T14:35:00Z", notes[0]["updated_at"])
	assert.Equal(t, "", notes[1]["title"]) // null title renders empty
	assert.Equal(t, "not_eee555fff666", notes[2]["note_id"])
	assert.NotContains(t, notes[0], "id")
}

func TestNoteListPassesFilters(t *testing.T) {
	var log requestLog
	srv := newNotesServer(t, &log)
	_, root, out := newGranolaEnv(t)
	stubNotes(t, newHTTPService(srv.Client(), srv.URL))

	_, err := execute(t, root, out,
		"granola", "note", "list", "--format", "json",
		"--created-after", "2026-08-01",
		"--created-before", "2026-09-01",
		"--updated-after", "2026-08-15T00:00:00Z",
		"--folder-id", "fol_abc123def456")
	require.NoError(t, err)

	queries := log.all()
	require.NotEmpty(t, queries)
	q := queries[0]
	assert.Equal(t, []string{"2026-08-01"}, q["created_after"])
	assert.Equal(t, []string{"2026-09-01"}, q["created_before"])
	assert.Equal(t, []string{"2026-08-15T00:00:00Z"}, q["updated_after"])
	assert.Equal(t, []string{"fol_abc123def456"}, q["folder_id"])
}

func TestNoteListTable(t *testing.T) {
	var log requestLog
	srv := newNotesServer(t, &log)
	_, root, out := newGranolaEnv(t)
	stubNotes(t, newHTTPService(srv.Client(), srv.URL))

	stdout, err := execute(t, root, out, "granola", "note", "list", "--format", "table")
	require.NoError(t, err)
	// go-pretty's StyleLight upper-cases header cells.
	assert.Contains(t, stdout, "NOTE_ID")
	assert.Contains(t, stdout, "TITLE")
	assert.Contains(t, stdout, "OWNER")
	assert.Contains(t, stdout, "not_aaa111bbb222")
	assert.Contains(t, stdout, "not_eee555fff666")
}

func TestNoteListDialErrorNamesGranola(t *testing.T) {
	// No stubbed dial, no accounts: the resolver must fail naming the
	// provider.
	_, root, out := newGranolaEnv(t)
	_, err := execute(t, root, out, "granola", "note", "list", "--format", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "granola")
}
