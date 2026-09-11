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

// userFixture loads a recorded users.* response from testdata. The tests are
// hermetic: fixtures are served by httptest, never the network. It is shared
// by user_get_test.go and user_list_test.go, the two user leaf suites.
func userFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	require.NoError(t, err)
	return data
}

// userRequestLog records the query params of every request the user leaves
// make, so tests can pin cursor paging and the page limit hermetically.
// Shared by user_get_test.go and user_list_test.go.
type userRequestLog struct {
	mu      sync.Mutex
	queries []map[string][]string
}

func (l *userRequestLog) add(r *http.Request) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.queries = append(l.queries, map[string][]string(r.URL.Query()))
}

func (l *userRequestLog) all() []map[string][]string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]map[string][]string{}, l.queries...)
}

// newUserGetServer serves one recorded users.info response and records every
// request.
func newUserGetServer(t *testing.T, body []byte) (*httptest.Server, *userRequestLog) {
	t.Helper()
	log := &userRequestLog{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.add(r)
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/users.info" {
			http.Error(w, `{"ok":false,"error":"unknown_path"}`, http.StatusNotFound)
			return
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv, log
}

// TestUserGetJSON pins the users.info mapping: id, name, and real_name come
// from the top-level user object, display_name from the nested profile;
// unmapped wire fields are ignored.
func TestUserGetJSON(t *testing.T) {
	srv, log := newUserGetServer(t, userFixture(t, "user_info.json"))
	_, root, out := newSlackEnv(t)
	stubDial(t, newHTTPService(srv.Client(), srv.URL))

	stdout, err := execute(t, root, out, "slack", "user", "get", "--user", "U02H6ECK2", "--format", "json")
	require.NoError(t, err)

	var user map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &user))
	assert.Equal(t, "U02H6ECK2", user["id"])
	assert.Equal(t, "oskar", user["name"])
	assert.Equal(t, "Oskar Hane", user["real_name"])
	assert.Equal(t, "oskarh", user["display_name"])
	assert.NotContains(t, user, "profile", "the wire profile object must not leak into output")
	assert.NotContains(t, user, "future_top_level")

	queries := log.all()
	require.Len(t, queries, 1)
	assert.Equal(t, []string{"U02H6ECK2"}, queries[0]["user"])
}

// TestUserGetFallsBackToTopLevelRealName: a wire user without a profile
// object still maps real_name from the top-level field; display_name stays
// empty because Slack only carries it under profile.
func TestUserGetFallsBackToTopLevelRealName(t *testing.T) {
	srv, _ := newUserGetServer(t, userFixture(t, "user_info_no_profile.json"))
	_, root, out := newSlackEnv(t)
	stubDial(t, newHTTPService(srv.Client(), srv.URL))

	stdout, err := execute(t, root, out, "slack", "user", "get", "--user", "U0BOT12345", "--format", "json")
	require.NoError(t, err)

	var user map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &user))
	assert.Equal(t, "U0BOT12345", user["id"])
	assert.Equal(t, "botskar", user["name"])
	assert.Equal(t, "Botskar Bot", user["real_name"])
	assert.Equal(t, "", user["display_name"])
}

// TestUserGetTable pins the UPPER-CASE table headers of the shared user
// output shape.
func TestUserGetTable(t *testing.T) {
	srv, _ := newUserGetServer(t, userFixture(t, "user_info.json"))
	_, root, out := newSlackEnv(t)
	stubDial(t, newHTTPService(srv.Client(), srv.URL))

	stdout, err := execute(t, root, out, "slack", "user", "get", "--user", "U02H6ECK2", "--format", "table")
	require.NoError(t, err)
	for _, header := range []string{"ID", "NAME", "REAL_NAME", "DISPLAY_NAME"} {
		assert.Contains(t, stdout, header)
	}
	assert.Contains(t, stdout, "oskar")
	assert.Contains(t, stdout, "Oskar Hane")
	assert.Contains(t, stdout, "oskarh")
}

// TestUserGetRequiresUserFlag: --user is mandatory, so get never runs with
// an empty ID.
func TestUserGetRequiresUserFlag(t *testing.T) {
	_, root, out := newSlackEnv(t)
	_, err := execute(t, root, out, "slack", "user", "get")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `required flag(s) "user" not set`)
}

// TestUserGetAPIOKFalseCarriesCode: an HTTP-200 ok:false answer surfaces the
// raw Slack error code in the command error.
func TestUserGetAPIOKFalseCarriesCode(t *testing.T) {
	srv, _ := newUserGetServer(t, []byte(`{"ok":false,"error":"user_not_found"}`))
	_, root, out := newSlackEnv(t)
	stubDial(t, newHTTPService(srv.Client(), srv.URL))

	_, err := execute(t, root, out, "slack", "user", "get", "--user", "U0MISSING", "--format", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "user_not_found")
}
