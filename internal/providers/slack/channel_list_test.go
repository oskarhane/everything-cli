package slack

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newChannelListServer serves the two recorded conversations.list pages keyed
// by the cursor param; an unknown path or cursor fails loudly as a Slack
// ok:false response.
func newChannelListServer(t *testing.T, log *channelRequestLog) *httptest.Server {
	t.Helper()
	page1 := channelFixture(t, "channel_list_page1.json")
	page2 := channelFixture(t, "channel_list_page2.json")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.add(r)
		if r.URL.Path != "/conversations.list" {
			http.Error(w, `{"ok":false,"error":"unknown_method"}`, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("cursor") {
		case "":
			_, _ = w.Write(page1)
		case "cursor-channel-2":
			_, _ = w.Write(page2)
		default:
			_, _ = w.Write([]byte(`{"ok":false,"error":"invalid_cursor"}`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestChannelListDefaultsTypesAndFollowsCursor: --types defaults to every
// conversation kind, the cursor is followed across both fixture pages, im
// entries carry the counterpart user field, and other channels omit it.
func TestChannelListDefaultsTypesAndFollowsCursor(t *testing.T) {
	var log channelRequestLog
	srv := newChannelListServer(t, &log)
	_, root, out := newSlackEnv(t)
	stubDial(t, newHTTPService(srv.Client(), srv.URL))

	stdout, err := execute(t, root, out, "slack", "channel", "list", "--format", "json")
	require.NoError(t, err)

	var channels []map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &channels))
	require.Len(t, channels, 4, "both pages are followed under the default --max")

	assert.Equal(t, "C0B3HMXFEUV", channels[0]["id"])
	assert.Equal(t, "general", channels[0]["name"])
	assert.Equal(t, false, channels[0]["is_private"])
	assert.NotContains(t, channels[0], "user", "user is im-only")
	assert.Equal(t, "D0B3HMXFEUW", channels[1]["id"])
	assert.Equal(t, "U02H6ECK2", channels[1]["user"], "im entries carry the counterpart user")
	assert.Equal(t, true, channels[2]["is_private"])

	queries := log.all()
	require.Len(t, queries, 2)
	assert.Equal(t, []string{"public_channel,private_channel,im,mpim"}, queries[0]["types"])
	assert.Equal(t, []string{"25"}, queries[0]["limit"])
	assert.Empty(t, queries[0]["cursor"])
	assert.Equal(t, []string{"public_channel,private_channel,im,mpim"}, queries[1]["types"])
	assert.Equal(t, []string{"23"}, queries[1]["limit"], "the second page asks only for the remaining budget")
	assert.Equal(t, []string{"cursor-channel-2"}, queries[1]["cursor"])
}

// TestChannelListExplicitTypes: an explicit --types value is passed through
// verbatim, and a filled --max budget stops paging.
func TestChannelListExplicitTypes(t *testing.T) {
	var log channelRequestLog
	srv := newChannelListServer(t, &log)
	_, root, out := newSlackEnv(t)
	stubDial(t, newHTTPService(srv.Client(), srv.URL))

	stdout, err := execute(t, root, out, "slack", "channel", "list",
		"--types", "private_channel", "--max", "2", "--format", "json")
	require.NoError(t, err)

	var channels []any
	require.NoError(t, json.Unmarshal([]byte(stdout), &channels))
	require.Len(t, channels, 2)

	queries := log.all()
	require.Len(t, queries, 1, "the filled budget stops paging")
	assert.Equal(t, []string{"private_channel"}, queries[0]["types"])
	assert.Equal(t, []string{"2"}, queries[0]["limit"])
}

// TestChannelListTable: go-pretty upper-cases header cells and the im user
// renders as a cell.
func TestChannelListTable(t *testing.T) {
	var log channelRequestLog
	srv := newChannelListServer(t, &log)
	_, root, out := newSlackEnv(t)
	stubDial(t, newHTTPService(srv.Client(), srv.URL))

	stdout, err := execute(t, root, out, "slack", "channel", "list", "--max", "0", "--format", "table")
	require.NoError(t, err)
	for _, header := range []string{"ID", "NAME", "IS_PRIVATE", "USER"} {
		assert.Contains(t, stdout, header)
	}
	assert.Contains(t, stdout, "general")
	assert.Contains(t, stdout, "secret-project")
	assert.Contains(t, stdout, "U02H6ECK2")
}

// TestChannelListChannelNotFound: an ok:false channel_not_found answer exits
// non-zero with the raw code visible.
func TestChannelListChannelNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":false,"error":"channel_not_found"}`))
	}))
	t.Cleanup(srv.Close)
	_, root, out := newSlackEnv(t)
	stubDial(t, newHTTPService(srv.Client(), srv.URL))

	_, err := execute(t, root, out, "slack", "channel", "list", "--format", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "channel_not_found")
}
