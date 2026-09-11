package slack

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The fixture channel and parent timestamp the two recorded thread pages and
// every command test agree on.
const (
	threadFixtureChannel  = "C0B3HMXFEUV"
	threadFixtureParentTS = "1726038000.000100"
)

// threadFixture loads a recorded conversations.replies response from
// testdata. The fixtures are served by httptest, never the network.
func threadFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	require.NoError(t, err)
	return data
}

// threadRequestLog records the query params of every request a test server
// receives, so assertions can pin the wire call shape.
type threadRequestLog struct {
	mu      sync.Mutex
	queries []url.Values
}

func (l *threadRequestLog) add(r *http.Request) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.queries = append(l.queries, r.URL.Query())
}

func (l *threadRequestLog) all() []url.Values {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]url.Values{}, l.queries...)
}

// newThreadService serves the recorded two-page thread keyed by the cursor
// parameter. Page one ends with next_cursor "thread-page-2" and page two with
// an empty one, so a well-behaved caller makes exactly two requests.
func newThreadService(t *testing.T, log *threadRequestLog) *httpService {
	t.Helper()
	page1 := threadFixture(t, "thread_page1.json")
	page2 := threadFixture(t, "thread_page2.json")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.add(r)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path != "/conversations.replies":
			http.Error(w, `{"ok":false,"error":"unknown_path"}`, http.StatusNotFound)
		case r.URL.Query().Get("cursor") == "":
			_, _ = w.Write(page1)
		case r.URL.Query().Get("cursor") == "thread-page-2":
			_, _ = w.Write(page2)
		default:
			http.Error(w, `{"ok":false,"error":"unknown_cursor"}`, http.StatusBadRequest)
		}
	}))
	t.Cleanup(srv.Close)
	return newHTTPService(srv.Client(), srv.URL)
}

// TestThreadJSONPreservesSlackOrder: with no cap the json output is the
// parent message followed by its replies in Slack order, each mapped to the
// shared Message shape with channel_id filled from the request.
func TestThreadJSONPreservesSlackOrder(t *testing.T) {
	_, root, out := newSlackEnv(t)
	var log threadRequestLog
	stubDial(t, newThreadService(t, &log))

	stdout, err := execute(t, root, out, "slack", "thread",
		"--channel", threadFixtureChannel, "--ts", threadFixtureParentTS,
		"--max", "0", "--format", "json")
	require.NoError(t, err)
	require.Len(t, log.all(), 2, "no cap must follow both pages and stop at the empty cursor")

	var messages []Message
	require.NoError(t, json.Unmarshal([]byte(stdout), &messages))
	require.Len(t, messages, 5)

	gotTS := make([]string, 0, len(messages))
	for _, m := range messages {
		gotTS = append(gotTS, m.TS)
	}
	assert.Equal(t, []string{
		threadFixtureParentTS,
		"1726038060.000200",
		"1726038120.000250",
		"1726038180.000300",
		"1726038240.000350",
	}, gotTS)
	assert.Contains(t, stdout, `"channel_id": "`+threadFixtureChannel+`"`)

	parent := messages[0]
	assert.Equal(t, threadFixtureChannel, parent.ChannelID)
	assert.Equal(t, "U02H6ECK2", parent.User)
	assert.Equal(t, "Deploy is done, monitoring the dashboards.", parent.Text)
	assert.Equal(t, threadFixtureParentTS, parent.ThreadTS)
	assert.Equal(t, 4, parent.ReplyCount)
	assert.True(t, parent.Edited)
	require.Len(t, parent.Reactions, 1)
	assert.Equal(t, Reaction{Name: "eyes", Count: 2}, parent.Reactions[0])

	reply := messages[1]
	assert.Equal(t, threadFixtureChannel, reply.ChannelID)
	assert.Equal(t, "U0GRACE", reply.User)
	assert.Equal(t, threadFixtureParentTS, reply.ThreadTS)
	assert.False(t, reply.Edited)
	assert.Empty(t, reply.Reactions)
}

// TestThreadMaxCapsItemsAcrossPages: --max budgets the total item count
// across cursor pages; 0 and the default's 25 both cover the whole thread.
func TestThreadMaxCapsItemsAcrossPages(t *testing.T) {
	cases := []struct {
		name      string
		max       string
		wantItems int
		wantPages int
	}{
		{"zero means no cap", "0", 5, 2},
		{"default cap covers every page", "25", 5, 2},
		{"cap inside the first page", "3", 3, 1},
		{"cap spans a page boundary", "4", 4, 2},
		{"cap of one is the parent only", "1", 1, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, root, out := newSlackEnv(t)
			var log threadRequestLog
			stubDial(t, newThreadService(t, &log))

			stdout, err := execute(t, root, out, "slack", "thread",
				"--channel", threadFixtureChannel, "--ts", threadFixtureParentTS,
				"--max", tc.max, "--format", "json")
			require.NoError(t, err)

			var messages []Message
			require.NoError(t, json.Unmarshal([]byte(stdout), &messages))
			require.Len(t, messages, tc.wantItems)
			assert.Equal(t, threadFixtureParentTS, messages[0].TS, "the parent always leads")
			assert.Len(t, log.all(), tc.wantPages)
		})
	}
}

// TestThreadRequestParamsCarryBudgetAndCursor: every call carries channel, ts
// and limit; the follow-up page carries the cursor and asks only for the
// remaining budget.
func TestThreadRequestParamsCarryBudgetAndCursor(t *testing.T) {
	_, root, out := newSlackEnv(t)
	var log threadRequestLog
	stubDial(t, newThreadService(t, &log))

	_, err := execute(t, root, out, "slack", "thread",
		"--channel", threadFixtureChannel, "--ts", threadFixtureParentTS,
		"--max", "4", "--format", "json")
	require.NoError(t, err)

	queries := log.all()
	require.Len(t, queries, 2)
	assert.Equal(t, threadFixtureChannel, queries[0].Get("channel"))
	assert.Equal(t, threadFixtureParentTS, queries[0].Get("ts"))
	assert.Equal(t, "4", queries[0].Get("limit"))
	assert.Empty(t, queries[0].Get("cursor"))

	assert.Equal(t, "1", queries[1].Get("limit"), "the second page asks only for the remaining budget")
	assert.Equal(t, "thread-page-2", queries[1].Get("cursor"))
}

// TestThreadRequiresChannelAndTS: both selectors are required flags; a
// missing one fails validation before any request is dialed.
func TestThreadRequiresChannelAndTS(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"neither flag", nil, `required flag(s) "channel", "ts" not set`},
		{"channel only", []string{"--channel", threadFixtureChannel}, `required flag(s) "ts" not set`},
		{"ts only", []string{"--ts", threadFixtureParentTS}, `required flag(s) "channel" not set`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, root, out := newSlackEnv(t)

			args := append([]string{"slack", "thread"}, tc.args...)
			_, err := execute(t, root, out, args...)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

// TestThreadTableHeadersUpperCase: go-pretty's StyleLight upper-cases header
// cells, so table output carries the UPPER-CASE field names.
func TestThreadTableHeadersUpperCase(t *testing.T) {
	_, root, out := newSlackEnv(t)
	var log threadRequestLog
	stubDial(t, newThreadService(t, &log))

	stdout, err := execute(t, root, out, "slack", "thread",
		"--channel", threadFixtureChannel, "--ts", threadFixtureParentTS, "--format", "table")
	require.NoError(t, err)
	for _, header := range []string{"TS", "USER", "TEXT", "THREAD_TS", "REPLY_COUNT", "EDITED"} {
		assert.Contains(t, stdout, header)
	}
	assert.Contains(t, stdout, "Deploy is done")
}

// TestThreadAPIErrorSurfacesCode: an ok:false response makes the leaf exit
// non-zero with Slack's raw error code visible.
func TestThreadAPIErrorSurfacesCode(t *testing.T) {
	_, root, out := newSlackEnv(t)
	notFound := threadFixture(t, "thread_not_found.json")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(notFound)
	}))
	t.Cleanup(srv.Close)
	stubDial(t, newHTTPService(srv.Client(), srv.URL))

	_, err := execute(t, root, out, "slack", "thread",
		"--channel", threadFixtureChannel, "--ts", threadFixtureParentTS, "--format", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "channel_not_found")
	var apiErr *apiError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, "channel_not_found", apiErr.Code)
}

// TestThreadRepliesCursorLoopIsBounded: an endpoint that keeps answering a
// non-empty next_cursor cannot loop forever; the shared page cap stops it.
func TestThreadRepliesCursorLoopIsBounded(t *testing.T) {
	const body = `{"ok":true,"messages":[{"ts":"1.000100","user":"U1","text":"x"}],"response_metadata":{"next_cursor":"always"}}`
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	svc := newHTTPService(srv.Client(), srv.URL)

	messages, err := svc.ThreadReplies(context.Background(), threadFixtureChannel, threadFixtureParentTS, 0)
	require.NoError(t, err)
	assert.Len(t, messages, maxListPages)
	assert.EqualValues(t, maxListPages, calls.Load())
}
