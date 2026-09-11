package slack

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// channelRequestLog records the query params of every request a channel test
// server receives, so tests can pin what was sent without touching the wire.
type channelRequestLog struct {
	mu      sync.Mutex
	queries []map[string][]string
}

func (l *channelRequestLog) add(r *http.Request) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.queries = append(l.queries, map[string][]string(r.URL.Query()))
}

func (l *channelRequestLog) all() []map[string][]string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]map[string][]string{}, l.queries...)
}

// channelFixture loads a recorded JSON response from testdata. The tests are
// hermetic: fixtures are served by httptest, never the network.
func channelFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	require.NoError(t, err)
	return data
}

// newChannelHistoryServer serves the two recorded conversations.history pages
// keyed by the cursor param; an unknown path or cursor fails loudly as a Slack
// ok:false response.
func newChannelHistoryServer(t *testing.T, log *channelRequestLog) *httptest.Server {
	t.Helper()
	page1 := channelFixture(t, "channel_history_page1.json")
	page2 := channelFixture(t, "channel_history_page2.json")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.add(r)
		if r.URL.Path != "/conversations.history" {
			http.Error(w, `{"ok":false,"error":"unknown_method"}`, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("cursor") {
		case "":
			_, _ = w.Write(page1)
		case "cursor-history-2":
			_, _ = w.Write(page2)
		default:
			_, _ = w.Write([]byte(`{"ok":false,"error":"invalid_cursor"}`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestChannelHistoryPaginatesAndRespectsMax: the history leaf follows
// response_metadata.next_cursor across a two-page fixture, stops when --max
// items are collected, and passes the time bounds through. Items carry the
// shared Message shape with channel_id filled from the request.
func TestChannelHistoryPaginatesAndRespectsMax(t *testing.T) {
	var log channelRequestLog
	srv := newChannelHistoryServer(t, &log)
	_, root, out := newSlackEnv(t)
	stubDial(t, newHTTPService(srv.Client(), srv.URL))

	stdout, err := execute(t, root, out, "slack", "channel", "history",
		"--channel", "C0B3HMXFEUV",
		"--oldest", "1512085950.000216",
		"--latest", "1512090000.000000",
		"--max", "3",
		"--format", "json")
	require.NoError(t, err)

	var messages []map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &messages))
	require.Len(t, messages, 3, "--max truncates the overshooting page")

	first := messages[0]
	assert.Equal(t, "1512085950.000216", first["ts"])
	assert.Equal(t, "C0B3HMXFEUV", first["channel_id"], "channel_id is filled from the request")
	assert.Equal(t, "U02H6ECK2", first["user"])
	assert.Equal(t, "deploy is green", first["text"])
	assert.Equal(t, "1512085940.000100", first["thread_ts"])
	assert.Equal(t, float64(2), first["reply_count"])
	assert.Equal(t, true, first["edited"])
	reactions, ok := first["reactions"].([]any)
	require.True(t, ok, "reactions render as an array of name/count objects")
	require.Len(t, reactions, 1)
	reaction, ok := reactions[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "eyes", reaction["name"])
	assert.Equal(t, float64(3), reaction["count"])

	queries := log.all()
	require.Len(t, queries, 2, "two pages fetched, then the budget stops the loop")
	assert.Equal(t, []string{"C0B3HMXFEUV"}, queries[0]["channel"])
	assert.Equal(t, []string{"1512085950.000216"}, queries[0]["oldest"])
	assert.Equal(t, []string{"1512090000.000000"}, queries[0]["latest"])
	assert.Equal(t, []string{"3"}, queries[0]["limit"])
	assert.Empty(t, queries[0]["cursor"])
	assert.Equal(t, []string{"cursor-history-2"}, queries[1]["cursor"])
	assert.Equal(t, []string{"1"}, queries[1]["limit"], "the second page asks only for the remaining budget")
}

// TestChannelHistoryNoCapFollowsCursorToTheEnd: --max 0 collects every page
// until the cursor is empty, asking for full pages.
func TestChannelHistoryNoCapFollowsCursorToTheEnd(t *testing.T) {
	var log channelRequestLog
	srv := newChannelHistoryServer(t, &log)
	_, root, out := newSlackEnv(t)
	stubDial(t, newHTTPService(srv.Client(), srv.URL))

	stdout, err := execute(t, root, out, "slack", "channel", "history",
		"--channel", "C0B3HMXFEUV", "--max", "0", "--format", "json")
	require.NoError(t, err)

	var messages []any
	require.NoError(t, json.Unmarshal([]byte(stdout), &messages))
	require.Len(t, messages, 4, "--max 0 caps nothing")

	queries := log.all()
	require.Len(t, queries, 2)
	assert.Equal(t, []string{"200"}, queries[0]["limit"])
	assert.Equal(t, []string{"200"}, queries[1]["limit"])
	assert.Equal(t, []string{"cursor-history-2"}, queries[1]["cursor"])
	assert.Empty(t, queries[1]["oldest"], "no time bounds passed, no oldest param sent")
}

// TestChannelHistoryStopsAtMaxWithoutFetchingPageTwo: a filled budget stops
// the cursor loop before the next request.
func TestChannelHistoryStopsAtMaxWithoutFetchingPageTwo(t *testing.T) {
	var log channelRequestLog
	srv := newChannelHistoryServer(t, &log)
	_, root, out := newSlackEnv(t)
	stubDial(t, newHTTPService(srv.Client(), srv.URL))

	stdout, err := execute(t, root, out, "slack", "channel", "history",
		"--channel", "C0B3HMXFEUV", "--max", "2", "--format", "json")
	require.NoError(t, err)

	var messages []any
	require.NoError(t, json.Unmarshal([]byte(stdout), &messages))
	require.Len(t, messages, 2)
	require.Len(t, log.all(), 1, "the filled budget must stop paging before fetching page two")
}

// TestChannelHistoryTable: go-pretty upper-cases header cells; reactions join
// into one cell.
func TestChannelHistoryTable(t *testing.T) {
	var log channelRequestLog
	srv := newChannelHistoryServer(t, &log)
	_, root, out := newSlackEnv(t)
	stubDial(t, newHTTPService(srv.Client(), srv.URL))

	stdout, err := execute(t, root, out, "slack", "channel", "history",
		"--channel", "C0B3HMXFEUV", "--max", "0", "--format", "table")
	require.NoError(t, err)
	for _, header := range []string{"TS", "CHANNEL_ID", "USER", "TEXT", "THREAD_TS", "REPLY_COUNT", "REACTIONS", "EDITED"} {
		assert.Contains(t, stdout, header)
	}
	assert.Contains(t, stdout, "deploy is green")
	assert.Contains(t, stdout, "eyes:3")
}

// TestChannelHistoryChannelNotFound: an ok:false channel_not_found answer
// exits non-zero with the raw code visible and writes no output.
func TestChannelHistoryChannelNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":false,"error":"channel_not_found"}`))
	}))
	t.Cleanup(srv.Close)
	_, root, out := newSlackEnv(t)
	stubDial(t, newHTTPService(srv.Client(), srv.URL))

	stdout, err := execute(t, root, out, "slack", "channel", "history",
		"--channel", "C_MISSING", "--format", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "channel_not_found")
	assert.Empty(t, stdout, "an API error writes no output")
}

// TestChannelHistoryRequiresChannelFlag: cobra rejects the invocation before
// any account is dialed.
func TestChannelHistoryRequiresChannelFlag(t *testing.T) {
	_, root, out := newSlackEnv(t)
	_, err := execute(t, root, out, "slack", "channel", "history", "--format", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "channel")
}
