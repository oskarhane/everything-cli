package slack

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/auth"
	"github.com/oskarhane/everything-cli/internal/config"
)

// authTestOK is a successful auth.test answer carrying the workspace
// identity the validate hook maps into Account.Identity.
const authTestOK = `{"ok":true,"team":"Neo4j","team_id":"T0B5K0M1AAC","user":"oskar","user_id":"U02H6ECK2"}`

// stubAuthTest points the real validate hook's auth.test probe at an
// httptest server answering body, for the test's lifetime. The network is
// never reached.
func stubAuthTest(t *testing.T, body string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/auth.test", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	stubValidateBase(t, srv.URL)
}

// newStore returns the store the commands build: the injected in-memory FS
// with the same config-dir resolution as production.
func newStore(t *testing.T, cfg *app.Config) *config.Store {
	t.Helper()
	store, err := config.NewStore(cfg.Fs, "")
	require.NoError(t, err)
	return store
}

// stubValidate swaps the strategy's auth.test probe for the test's lifetime.
// The strategy's Validate closure reads validateKey at call time, so the swap
// reaches every add.
func stubValidate(t *testing.T, fn func(context.Context, string) (map[string]string, error)) {
	t.Helper()
	saved := validateKey
	validateKey = fn
	t.Cleanup(func() { validateKey = saved })
}

// seedAccount persists a Slack account with a fake token and workspace
// identity through the real strategy, stubbing the auth.test probe so the
// seed never reaches the network. The token is distinctive per test so
// assertions can prove it never reaches any output format.
func seedAccount(t *testing.T, cfg *app.Config, name, key string) {
	t.Helper()
	stubValidate(t, func(context.Context, string) (map[string]string, error) {
		return map[string]string{
			"team":    "Seed Team",
			"team_id": "TSEED",
			"user":    "seed-user",
			"user_id": "USEED",
		}, nil
	})
	_, err := strategy.Add(context.Background(), cfg.Fs, newStore(t, cfg), auth.AddOptions{Name: name, APIKey: key})
	require.NoError(t, err)
}
