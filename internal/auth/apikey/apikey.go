// Package apikey implements the auth.Strategy seam for providers whose API
// authenticates with a static key sent in an HTTP header — Linear's
// "Authorization: <key>", a "Bearer <key>" scheme, or a custom header such
// as "X-Api-Key". The header name and value format are per-provider
// configuration; the capture order (flag, env var, hidden prompt), the
// account persistence shape, and the redaction registration are shared
// machinery.
//
// The key is a secret on par with an OAuth refresh token: it is captured
// without echo, never printed, and registered with the redaction registry
// at the capture/read point (AGENTS.md rule) so any later output wiring
// scrubs it.
package apikey

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/oskarhane/everything-cli/internal/auth"
	"github.com/oskarhane/everything-cli/internal/config"
	"github.com/spf13/afero"
	"golang.org/x/term"
)

// Config pins one provider's API-key shape. Adding an API-key provider is
// configuration, not new machinery.
type Config struct {
	// Provider is the provider ID stored on each account record
	// ("linear", "granola"); it selects the store's per-provider
	// directory. Required.
	Provider string
	// HeaderName is the HTTP header carrying the key, e.g.
	// "Authorization" or "X-Api-Key". Required.
	HeaderName string
	// HeaderFormat formats the captured key into the header value. It
	// must be a literal containing exactly one %s verb, replaced by the
	// key: "%s" for a raw key (Linear), "Bearer %s" for a bearer scheme.
	// Required.
	HeaderFormat string
	// EnvVar names the environment variable consulted for the key when
	// no flag value is given ("LINEAR_API_KEY"); empty disables the env
	// source.
	EnvVar string
}

// Strategy is the auth.Strategy for API-key providers. The getenv and
// prompt seams exist for hermetic tests; production wiring gets the
// defaults (os.Getenv and a hidden terminal prompt).
type Strategy struct {
	cfg    Config
	getenv func(string) string
	prompt func() (string, error)
}

// Compile-time proof that Strategy satisfies the auth seam.
var _ auth.Strategy = (*Strategy)(nil)

// New returns the API-key Strategy for cfg, or an error when cfg is
// unusable (missing provider/header or a malformed header format).
func New(cfg Config) (*Strategy, error) {
	switch {
	case cfg.Provider == "":
		return nil, errors.New("apikey: provider is required")
	case cfg.HeaderName == "":
		return nil, errors.New("apikey: header name is required")
	case strings.Count(cfg.HeaderFormat, "%s") != 1 ||
		strings.Contains(strings.Replace(cfg.HeaderFormat, "%s", "", 1), "%"):
		return nil, fmt.Errorf("apikey: header format %q must be a literal with exactly one %%s verb", cfg.HeaderFormat)
	}
	return &Strategy{cfg: cfg, getenv: os.Getenv, prompt: promptHidden}, nil
}

// authPayload is the provider-shaped JSON stored in Account.Auth. The key
// field is snake_case per the casing rule.
type authPayload struct {
	APIKey string `json:"api_key"`
}

// Add captures the key — flag value, then env var, then a hidden prompt —
// and persists the account under the provider's store directory. The key
// is registered for redaction immediately at capture, before anything
// could print it.
func (s *Strategy) Add(_ context.Context, _ afero.Fs, store *config.Store, opts auth.AddOptions) (*config.Account, error) {
	key, err := s.capture(opts.APIKey)
	if err != nil {
		return nil, err
	}
	// Mint point (AGENTS.md rule): register before any output path exists.
	auth.RegisterSecret(key)
	payload, err := json.Marshal(authPayload{APIKey: key})
	if err != nil {
		return nil, fmt.Errorf("encoding auth payload: %w", err)
	}
	acct := &config.Account{
		Name:     opts.Name,
		Provider: s.cfg.Provider,
		Auth:     payload,
	}
	if err := store.Save(acct); err != nil {
		return nil, err
	}
	return store.GetProvider(s.cfg.Provider, acct.Name)
}

// capture resolves the key from the flag value, then the configured env
// var, then the hidden prompt.
func (s *Strategy) capture(flagValue string) (string, error) {
	key := strings.TrimSpace(flagValue)
	if key == "" && s.cfg.EnvVar != "" {
		key = strings.TrimSpace(s.getenv(s.cfg.EnvVar))
	}
	if key == "" {
		prompted, err := s.prompt()
		if err != nil {
			return "", err
		}
		key = strings.TrimSpace(prompted)
	}
	if key != "" {
		return key, nil
	}
	if s.cfg.EnvVar != "" {
		return "", fmt.Errorf("no API key: pass --api-key, set %s, or enter it at the hidden prompt", s.cfg.EnvVar)
	}
	return "", errors.New("no API key: pass --api-key or enter it at the hidden prompt")
}

// promptHidden reads the key from the terminal without echo. The prompt
// goes to stderr so stdout stays machine-readable; the typed key is never
// written anywhere.
func promptHidden() (string, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return "", errors.New("no API key and stdin is not a terminal: pass --api-key or set the provider's env var")
	}
	_, _ = fmt.Fprint(os.Stderr, "API key: ")
	raw, err := term.ReadPassword(fd)
	_, _ = fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("reading API key: %w", err)
	}
	return string(raw), nil
}

// Client builds an *http.Client wrapping http.DefaultClient whose
// transport sets the configured header from the stored key. The key read
// from disk is re-registered for redaction (read point).
func (s *Strategy) Client(_ context.Context, acct *config.Account) (*http.Client, error) {
	if acct == nil {
		return nil, errors.New("no account")
	}
	var payload authPayload
	if err := json.Unmarshal(acct.Auth, &payload); err != nil {
		return nil, fmt.Errorf("parsing account %q auth: %w", acct.Name, err)
	}
	if payload.APIKey == "" {
		return nil, fmt.Errorf("account %q holds no API key", acct.Name)
	}
	// Read point: a key restored from disk must be scrubbed from output too.
	auth.RegisterSecret(payload.APIKey)
	base := http.DefaultTransport
	if http.DefaultClient.Transport != nil {
		base = http.DefaultClient.Transport
	}
	client := *http.DefaultClient
	client.Transport = &headerTransport{
		name:  s.cfg.HeaderName,
		value: fmt.Sprintf(s.cfg.HeaderFormat, payload.APIKey),
		base:  base,
	}
	return &client, nil
}

// headerTransport sets one header on every request, cloning the request so
// the caller's is never mutated.
type headerTransport struct {
	name  string
	value string
	base  http.RoundTripper
}

func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r := req.Clone(req.Context())
	r.Header.Set(t.name, t.value)
	return t.base.RoundTrip(r)
}
