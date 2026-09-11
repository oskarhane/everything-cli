package slack

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubValidateBase points the validate hook's probe at an httptest server
// for the test's lifetime. The account node's tests stub validateKey itself.
func stubValidateBase(t *testing.T, url string) {
	t.Helper()
	saved := validateBaseURL
	validateBaseURL = url
	t.Cleanup(func() { validateBaseURL = saved })
}

// stubWarnWriter captures the validate hook's warnings without touching
// process stderr.
func stubWarnWriter(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	saved := warnWriter
	warnWriter = &buf
	t.Cleanup(func() { warnWriter = saved })
	return &buf
}

// TestValidateKeyMapsAuthTestIdentity: the hook authenticates /auth.test
// with the captured key and maps the workspace identity fields.
func TestValidateKeyMapsAuthTestIdentity(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/auth.test", r.URL.Path)
		assert.Equal(t, "Bearer xoxp-user-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"ok": true,
			"url": "https://example.slack.com/",
			"team": "Hanelabs",
			"user": "oskar",
			"team_id": "T0B5K0M1AAC",
			"user_id": "U02H6ECK2",
			"future_field": {"nested": 1}
		}`))
	}))
	t.Cleanup(srv.Close)
	stubValidateBase(t, srv.URL)
	warned := stubWarnWriter(t)

	identity, err := validateKey(context.Background(), "xoxp-user-token")
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"team":    "Hanelabs",
		"team_id": "T0B5K0M1AAC",
		"user":    "oskar",
		"user_id": "U02H6ECK2",
	}, identity)
	assert.Empty(t, warned.String())
}

// TestValidateKeyWarnsOnNonUserToken: a bot/app token cannot call
// search.messages, so the hook warns (mentioning xoxp) but still validates.
func TestValidateKeyWarnsOnNonUserToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"team":"Hanelabs","team_id":"T0B5K0M1AAC","user":"botskar","user_id":"U0BOT"}`))
	}))
	t.Cleanup(srv.Close)
	stubValidateBase(t, srv.URL)
	warned := stubWarnWriter(t)

	identity, err := validateKey(context.Background(), "xoxb-bot-token")
	require.NoError(t, err)
	assert.Equal(t, "U0BOT", identity["user_id"])
	assert.Contains(t, warned.String(), "xoxp")
}

// TestValidateKeySurfacesAuthTestCode: a rejected token surfaces Slack's
// raw error code through the shared apiCall mapping.
func TestValidateKeySurfacesAuthTestCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":false,"error":"invalid_auth"}`))
	}))
	t.Cleanup(srv.Close)
	stubValidateBase(t, srv.URL)
	stubWarnWriter(t)

	_, err := validateKey(context.Background(), "xoxp-bad-token")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid_auth")
}
