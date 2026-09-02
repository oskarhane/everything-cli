package apikey

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oskarhane/everything-cli/internal/auth"
	"github.com/oskarhane/everything-cli/internal/config"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// linearConfig shapes Linear's raw-key Authorization header; bearerConfig
// shapes a Bearer scheme; xapiKeyConfig shapes a custom header with no env
// var. Keys in tests are obviously fake.
var (
	linearConfig   = Config{Provider: "linear", HeaderName: "Authorization", HeaderFormat: "%s", EnvVar: "LINEAR_API_KEY"}
	bearerConfig   = Config{Provider: "granola", HeaderName: "Authorization", HeaderFormat: "Bearer %s", EnvVar: "GRANOLA_API_KEY"}
	xapiKeyConfig  = Config{Provider: "acme", HeaderName: "X-Api-Key", HeaderFormat: "%s", EnvVar: ""}
	errPromptBroke = errors.New("prompt broke")
)

func newTestStrategy(t *testing.T, cfg Config) *Strategy {
	t.Helper()
	s, err := New(cfg)
	require.NoError(t, err)
	return s
}

func newTestStore(t *testing.T) (*config.Store, afero.Fs) {
	t.Helper()
	fs := afero.NewMemMapFs()
	store, err := config.NewStore(fs, "/cfg")
	require.NoError(t, err)
	return store, fs
}

func TestNewValidatesConfig(t *testing.T) {
	for _, tt := range []struct {
		name string
		cfg  Config
	}{
		{"empty provider", Config{HeaderName: "Authorization", HeaderFormat: "%s"}},
		{"empty header name", Config{Provider: "linear", HeaderFormat: "%s"}},
		{"format without verb", Config{Provider: "linear", HeaderName: "Authorization", HeaderFormat: "Bearer"}},
		{"format with two verbs", Config{Provider: "linear", HeaderName: "Authorization", HeaderFormat: "%s %s"}},
		{"format with foreign verb", Config{Provider: "linear", HeaderName: "Authorization", HeaderFormat: "%s %d"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.cfg)
			assert.Error(t, err)
		})
	}
	for _, cfg := range []Config{linearConfig, bearerConfig, xapiKeyConfig} {
		_, err := New(cfg)
		assert.NoError(t, err, cfg.Provider)
	}
}

// TestStrategySatisfiesSeam pins the compile-time contract: an apikey
// Strategy is an auth.Strategy, configurable per provider.
func TestStrategySatisfiesSeam(t *testing.T) {
	for _, cfg := range []Config{linearConfig, bearerConfig, xapiKeyConfig} {
		s := newTestStrategy(t, cfg)
		var seam auth.Strategy = s
		assert.NotNil(t, seam)
	}
}

// TestAddCapturesFromFlag: a flag-provided key is stored provider-scoped
// with the key inside Account.Auth, and env/prompt are never consulted.
func TestAddCapturesFromFlag(t *testing.T) {
	s := newTestStrategy(t, linearConfig)
	s.getenv = func(string) string {
		t.Fatal("getenv must not be consulted when the flag supplies the key")
		return ""
	}
	s.prompt = func() (string, error) {
		t.Fatal("prompt must not be consulted when the flag supplies the key")
		return "", nil
	}
	store, fs := newTestStore(t)

	acct, err := s.Add(context.Background(), fs, store, auth.AddOptions{Name: "work", APIKey: "test-key-123"})
	require.NoError(t, err)
	assert.Equal(t, "work", acct.Name)
	assert.Equal(t, "linear", acct.Provider)
	assert.JSONEq(t, `{"api_key":"test-key-123"}`, string(acct.Auth))

	raw, err := afero.ReadFile(fs, "/cfg/accounts/linear/work.json")
	require.NoError(t, err, "the account lands nested per provider")
	assert.Contains(t, string(raw), `"api_key"`)
}

// TestAddCapturesFromEnv: with no flag value, the configured env var
// supplies the key and the prompt stays untouched.
func TestAddCapturesFromEnv(t *testing.T) {
	s := newTestStrategy(t, bearerConfig)
	s.getenv = func(name string) string {
		assert.Equal(t, "GRANOLA_API_KEY", name)
		return "test-key-456"
	}
	s.prompt = func() (string, error) {
		t.Fatal("prompt must not be consulted when the env var supplies the key")
		return "", nil
	}
	store, fs := newTestStore(t)

	acct, err := s.Add(context.Background(), fs, store, auth.AddOptions{Name: "personal"})
	require.NoError(t, err)
	assert.JSONEq(t, `{"api_key":"test-key-456"}`, string(acct.Auth))
}

// TestAddCapturesFromHiddenPrompt: with neither flag nor env, the hidden
// prompt supplies the key.
func TestAddCapturesFromHiddenPrompt(t *testing.T) {
	s := newTestStrategy(t, xapiKeyConfig)
	s.prompt = func() (string, error) { return "test-key-789", nil }
	store, fs := newTestStore(t)

	acct, err := s.Add(context.Background(), fs, store, auth.AddOptions{Name: "main"})
	require.NoError(t, err)
	assert.JSONEq(t, `{"api_key":"test-key-789"}`, string(acct.Auth))
}

func TestAddNoKeyAvailable(t *testing.T) {
	t.Run("with env var configured", func(t *testing.T) {
		s := newTestStrategy(t, linearConfig)
		s.getenv = func(string) string { return "" }
		s.prompt = func() (string, error) { return "  ", nil }
		store, fs := newTestStore(t)
		_, err := s.Add(context.Background(), fs, store, auth.AddOptions{Name: "work"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "LINEAR_API_KEY", "the error names the env var, never a key value")
	})
	t.Run("prompt failure propagates", func(t *testing.T) {
		s := newTestStrategy(t, linearConfig)
		s.getenv = func(string) string { return "" }
		s.prompt = func() (string, error) { return "", errPromptBroke }
		store, fs := newTestStore(t)
		_, err := s.Add(context.Background(), fs, store, auth.AddOptions{Name: "work"})
		assert.ErrorIs(t, err, errPromptBroke)
	})
}

// TestClientSetsConfiguredHeader: each provider config yields a client
// whose transport sets that provider's header from the stored key.
func TestClientSetsConfiguredHeader(t *testing.T) {
	for _, tt := range []struct {
		name       string
		cfg        Config
		wantHeader string
		wantValue  string
	}{
		{"raw key", linearConfig, "Authorization", "test-key-123"},
		{"bearer format", bearerConfig, "Authorization", "Bearer test-key-123"},
		{"custom header", xapiKeyConfig, "X-Api-Key", "test-key-123"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var gotHeader, gotValue string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotHeader = tt.wantHeader
				gotValue = r.Header.Get(tt.wantHeader)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			s := newTestStrategy(t, tt.cfg)
			payload, err := json.Marshal(authPayload{APIKey: "test-key-123"})
			require.NoError(t, err)
			client, err := s.Client(context.Background(), &config.Account{
				Name: "work", Provider: tt.cfg.Provider, Auth: payload,
			})
			require.NoError(t, err)

			req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
			require.NoError(t, err)
			resp, err := client.Do(req)
			require.NoError(t, err)
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()

			assert.Equal(t, tt.wantHeader, gotHeader)
			assert.Equal(t, tt.wantValue, gotValue)
			assert.Empty(t, req.Header.Get(tt.wantHeader), "the caller's request is never mutated")
		})
	}
}

func TestClientRejectsUnusableAccount(t *testing.T) {
	s := newTestStrategy(t, linearConfig)
	_, err := s.Client(context.Background(), nil)
	assert.Error(t, err, "nil account")
	_, err = s.Client(context.Background(), &config.Account{Name: "work", Auth: json.RawMessage(`{`)})
	assert.Error(t, err, "malformed auth payload")
	_, err = s.Client(context.Background(), &config.Account{Name: "work", Auth: json.RawMessage(`{}`)})
	assert.Error(t, err, "payload without a key")
}

// TestKeyRegisteredForRedaction pins the AGENTS.md mint/read-point rule at
// the strategy level: after Add captures a key (and after Client reads it
// back from disk), the redactor scrubs that key from any output — so an
// account get rendering that passes through Redact can never leak it.
func TestKeyRegisteredForRedaction(t *testing.T) {
	s := newTestStrategy(t, linearConfig)
	s.getenv = func(string) string { return "" }
	store, fs := newTestStore(t)

	// Inside the prompt the key exists but Add has not captured it yet;
	// nothing has had a chance to print it.
	s.prompt = func() (string, error) {
		assert.NotContains(t, auth.Redact("test-key-redact"), "***",
			"registration happens at capture, not before the key exists")
		return "test-key-redact", nil
	}

	_, err := s.Add(context.Background(), fs, store, auth.AddOptions{Name: "work"})
	require.NoError(t, err)

	// After Add, any output string carrying the key is scrubbed.
	out := auth.Redact("name: work\nauth:\n  api_key: test-key-redact\n")
	assert.NotContains(t, out, "test-key-redact")
	assert.Contains(t, out, "***")

	// The persisted account document — what a generic account get would
	// render — is safe once redacted.
	raw, err := afero.ReadFile(fs, "/cfg/accounts/linear/work.json")
	require.NoError(t, err)
	scrubbed := auth.Redact(string(raw))
	assert.NotContains(t, scrubbed, "test-key-redact")

	// After Client re-reads the key from disk (a fresh process in real
	// use), the redactor still scrubs it.
	_, err = s.Client(context.Background(), &config.Account{
		Name: "work", Provider: "linear", Auth: json.RawMessage(`{"api_key":"test-key-client"}`),
	})
	require.NoError(t, err)
	assert.NotContains(t, auth.Redact("leak: test-key-client"), "test-key-client")
}
