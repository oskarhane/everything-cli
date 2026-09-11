package slack

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// searchTestFixture loads a recorded search.messages response from testdata.
// Tests are hermetic: fixtures are served by httptest, never the network.
func searchTestFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	require.NoError(t, err)
	return string(data)
}

// searchCall is one recorded search.messages request.
type searchCall struct {
	Path  string
	Query url.Values
}

// searchMessagesLog records every search.messages request the service makes,
// so tests can pin page/count pagination and pass-through parameters.
type searchMessagesLog struct {
	mu    sync.Mutex
	calls []searchCall
}

func (l *searchMessagesLog) add(r *http.Request) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.calls = append(l.calls, searchCall{Path: r.URL.Path, Query: r.URL.Query()})
}

func (l *searchMessagesLog) all() []searchCall {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]searchCall{}, l.calls...)
}

// newSearchMessagesServer serves recorded search.messages bodies keyed by the
// page query param. Unknown pages answer 400 so a pagination bug fails
// loudly instead of silently re-serving a fixture.
func newSearchMessagesServer(t *testing.T, log *searchMessagesLog, pages map[string]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if log != nil {
			log.add(r)
		}
		body, ok := pages[r.URL.Query().Get("page")]
		if !ok {
			http.Error(w, `{"ok":false,"error":"unexpected_page"}`, http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// stubSearchDial points the dial seam at the httptest-backed service.
func stubSearchDial(t *testing.T, srv *httptest.Server) {
	t.Helper()
	stubDial(t, newHTTPService(srv.Client(), srv.URL))
}

// TestSearchMessagesPagesUntilMaxCollected: the first request carries the
// 1-based page, count=100, the query, and --sort; the loop stops as soon as
// --max items are collected even though another page exists.
func TestSearchMessagesPagesUntilMaxCollected(t *testing.T) {
	var log searchMessagesLog
	srv := newSearchMessagesServer(t, &log, map[string]string{
		"1": searchTestFixture(t, "search_page1.json"),
		"2": searchTestFixture(t, "search_page2.json"),
	})
	_, root, out := newSlackEnv(t)
	stubSearchDial(t, srv)

	stdout, err := execute(t, root, out, "slack", "search", "messages",
		"--query", "from:me", "--sort", "timestamp", "--max", "2", "--format", "json")
	require.NoError(t, err)

	calls := log.all()
	require.Len(t, calls, 1, "max reached on page 1: page 2 must not be fetched")
	assert.Equal(t, "/search.messages", calls[0].Path)
	assert.Equal(t, "1", calls[0].Query.Get("page"))
	assert.Equal(t, "100", calls[0].Query.Get("count"))
	assert.Equal(t, "from:me", calls[0].Query.Get("query"))
	assert.Equal(t, "timestamp", calls[0].Query.Get("sort"))

	var payload struct {
		Messages []map[string]any `json:"messages"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &payload))
	assert.Len(t, payload.Messages, 2)
}

// TestSearchMessagesFollowsPagesAndTruncates: --max larger than one page
// follows the page counter, then trims the overshoot from the last page.
func TestSearchMessagesFollowsPagesAndTruncates(t *testing.T) {
	var log searchMessagesLog
	srv := newSearchMessagesServer(t, &log, map[string]string{
		"1": searchTestFixture(t, "search_page1.json"),
		"2": searchTestFixture(t, "search_page2.json"),
	})
	_, root, out := newSlackEnv(t)
	stubSearchDial(t, srv)

	stdout, err := execute(t, root, out, "slack", "search", "messages",
		"--query", "from:me", "--max", "3", "--format", "json")
	require.NoError(t, err)

	calls := log.all()
	require.Len(t, calls, 2)
	assert.Equal(t, "1", calls[0].Query.Get("page"))
	assert.Equal(t, "2", calls[1].Query.Get("page"))
	assert.Equal(t, "100", calls[1].Query.Get("count"))
	assert.Empty(t, calls[0].Query.Get("sort"), "no --sort means Slack's default (score)")

	var payload struct {
		Messages []map[string]any `json:"messages"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &payload))
	require.Len(t, payload.Messages, 3)
	assert.Equal(t, "1757480000.000300", payload.Messages[2]["ts"], "the fourth match is truncated")
}

// TestSearchMessagesNoCapFollowsAllPages: --max 0 collects across the last
// page (page >= pages ends the loop).
func TestSearchMessagesNoCapFollowsAllPages(t *testing.T) {
	var log searchMessagesLog
	srv := newSearchMessagesServer(t, &log, map[string]string{
		"1": searchTestFixture(t, "search_page1.json"),
		"2": searchTestFixture(t, "search_page2.json"),
	})
	_, root, out := newSlackEnv(t)
	stubSearchDial(t, srv)

	stdout, err := execute(t, root, out, "slack", "search", "messages",
		"--query", "from:me", "--max", "0", "--format", "json")
	require.NoError(t, err)

	require.Len(t, log.all(), 2)
	var payload struct {
		Messages []map[string]any `json:"messages"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &payload))
	assert.Len(t, payload.Messages, 4)
}

// TestSearchMessagesJSONEchoesQueryAndMatchFields: the JSON payload is
// {query, messages}; each match flattens channel_id/channel_name from the
// wire channel object and carries user, username, ts, text, permalink, and
// thread_ts only when the match is a thread reply.
func TestSearchMessagesJSONEchoesQueryAndMatchFields(t *testing.T) {
	var log searchMessagesLog
	srv := newSearchMessagesServer(t, &log, map[string]string{
		"1": searchTestFixture(t, "search_page1.json"),
	})
	_, root, out := newSlackEnv(t)
	stubSearchDial(t, srv)

	stdout, err := execute(t, root, out, "slack", "search", "messages",
		"--query", "from:me", "--max", "2", "--format", "json")
	require.NoError(t, err)

	var payload struct {
		Query    string           `json:"query"`
		Messages []map[string]any `json:"messages"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &payload))
	assert.Equal(t, "from:me", payload.Query)
	require.Len(t, payload.Messages, 2)

	first := payload.Messages[0]
	assert.Equal(t, "C0B3HMXFEUV", first["channel_id"])
	assert.Equal(t, "general", first["channel_name"])
	assert.Equal(t, "U02H6ECK2", first["user"])
	assert.Equal(t, "oskar", first["username"])
	assert.Equal(t, "1757500000.000100", first["ts"])
	assert.Equal(t, "deployed the ingest fix", first["text"])
	assert.Equal(t, "https://hanelabs.slack.com/archives/C0B3HMXFEUV/p1757500000000100", first["permalink"])
	assert.Equal(t, "1757499000.000050", first["thread_ts"])

	_, hasThread := payload.Messages[1]["thread_ts"]
	assert.False(t, hasThread, "thread_ts renders only when the match is a thread reply")
}

// TestSearchMessagesTableHeaders: go-pretty's StyleLight upper-cases header
// cells, one row per match.
func TestSearchMessagesTableHeaders(t *testing.T) {
	var log searchMessagesLog
	srv := newSearchMessagesServer(t, &log, map[string]string{
		"1": searchTestFixture(t, "search_page1.json"),
	})
	_, root, out := newSlackEnv(t)
	stubSearchDial(t, srv)

	stdout, err := execute(t, root, out, "slack", "search", "messages",
		"--query", "from:me", "--max", "1", "--format", "table")
	require.NoError(t, err)
	for _, header := range []string{"CHANNEL_ID", "CHANNEL_NAME", "USER", "USERNAME", "TS", "TEXT", "PERMALINK", "THREAD_TS", "FILES"} {
		assert.Contains(t, stdout, header)
	}
	assert.Contains(t, stdout, "deployed the ingest fix")
}

// TestSearchMessagesBotTokenSurfacesCode: the bot-token case — an ok:false
// not_allowed_token_type response — exits non-zero with the Slack code in
// the error line.
func TestSearchMessagesBotTokenSurfacesCode(t *testing.T) {
	var log searchMessagesLog
	srv := newSearchMessagesServer(t, &log, map[string]string{
		"1": searchTestFixture(t, "search_denied.json"),
	})
	_, root, out := newSlackEnv(t)
	stubSearchDial(t, srv)

	stdout, err := execute(t, root, out, "slack", "search", "messages",
		"--query", "from:me", "--format", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not_allowed_token_type")
	assert.Empty(t, stdout)
}

// TestSearchMessageRowSurfacesFilesOnlyWhenPresent: the search row carries the
// shared files cell (name:id) only when the match has attachments.
func TestSearchMessageRowSurfacesFilesOnlyWhenPresent(t *testing.T) {
	withFiles := searchMessageRow(SearchMatch{
		Message: Message{TS: "1.0", Files: []File{{ID: "F1", Name: "a.txt", Mimetype: "text/plain", Size: 1}}},
	})
	assert.Equal(t, "a.txt:F1", withFiles["files"].(fileCell).String())

	without := searchMessageRow(SearchMatch{Message: Message{TS: "1.0"}})
	_, hasFiles := without["files"]
	assert.False(t, hasFiles, "files are omitted when the match has no attachments")
}

// TestSearchMessagesRequiresQuery: omitting --query fails validation before
// any request is made.
func TestSearchMessagesRequiresQuery(t *testing.T) {
	var log searchMessagesLog
	srv := newSearchMessagesServer(t, &log, map[string]string{
		"1": searchTestFixture(t, "search_page1.json"),
	})
	_, root, out := newSlackEnv(t)
	stubSearchDial(t, srv)

	_, err := execute(t, root, out, "slack", "search", "messages", "--format", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `required flag(s) "query" not set`)
	assert.Empty(t, log.all())
}
