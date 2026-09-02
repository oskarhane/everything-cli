package linear

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/afero"
	"golang.org/x/oauth2"

	"github.com/oskarhane/everything-cli/internal/auth"
	"github.com/oskarhane/everything-cli/internal/config"
)

// Linear's OAuth and GraphQL endpoints, pinned by the provider. They are
// the only endpoints the OAuth path ever talks to; no user-supplied file
// can redirect them (auth.parseCredentials pins the profile's endpoint over
// any auth_uri/token_uri a credentials document claims).
const (
	linearAuthURL    = "https://linear.app/oauth/authorize"
	linearTokenURL   = "https://api.linear.app/oauth/token"
	linearGraphQLURL = "https://api.linear.app/graphql"
)

// Environment variables consulted for the OAuth app credentials when the
// --client-id/--client-secret flags are not given.
const (
	envVarClientID     = "LINEAR_CLIENT_ID"
	envVarClientSecret = "LINEAR_CLIENT_SECRET"
)

// linearOAuthProfile is Linear's OAuth profile: endpoints pinned per
// research-linear-api, scopes read,write (read + issue/comment writes),
// the comma scope separator Linear's authorize endpoint documents, and the
// read scope guaranteeing the viewer query can resolve the account email.
// IdentityResolver is attached per Add call so it can capture the viewer.
var linearOAuthProfile = auth.OAuthProfile{
	Name: "Linear",
	Endpoint: oauth2.Endpoint{
		AuthURL:  linearAuthURL,
		TokenURL: linearTokenURL,
	},
	UserinfoURL:    linearGraphQLURL,
	EmailScope:     "read",
	DefaultScopes:  []string{"read", "write"},
	ScopeSeparator: ",",
}

// viewer is the identity Linear's GraphQL `viewer` query resolves after
// the OAuth flow.
type viewer struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// identity maps the viewer onto the account's Identity field, omitting
// empty values.
func (v viewer) identity() map[string]string {
	out := map[string]string{}
	if v.ID != "" {
		out["id"] = v.ID
	}
	if v.Name != "" {
		out["name"] = v.Name
	}
	if v.Email != "" {
		out["email"] = v.Email
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// oauthAuthPayload is the provider-shaped JSON stored in Account.Auth for
// OAuth accounts: the app credentials the refreshing token source needs.
// client_id is non-secret metadata; client_secret is a secret, registered
// for redaction at mint/read.
type oauthAuthPayload struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret,omitempty"`
}

// OAuthStrategy is Linear's OAuth2 (authorization-code + PKCE, loopback
// redirect) auth.Strategy. It reuses the generalized flow
// (auth.RunFlowWith) for the browser flow and the generalized refreshing
// token source (auth.TokenSourceForProvider) for Client, against the
// pinned linearOAuthProfile; the only Linear-specific parts are the client
// credentials capture (flag/env, no credentials file) and identity
// resolution through the GraphQL viewer query.
type OAuthStrategy struct {
	profile    auth.OAuthProfile
	graphqlURL string
	fs         afero.Fs
	store      *config.Store
	getenv     func(string) string
	// runFlow is the flow seam; production is auth.RunFlowWith, tests a
	// hermetic flow against fake endpoints.
	runFlow func(fs afero.Fs, credentialsPath string, scopes []string, profile auth.OAuthProfile) (*oauth2.Token, string, error)
}

// Compile-time proof that OAuthStrategy satisfies the auth seam.
var _ auth.Strategy = (*OAuthStrategy)(nil)

// newOAuthStrategy builds the production OAuth strategy. fs and store back
// Client's token refresh and persistence; Add uses the fs/store it is
// handed per call.
func newOAuthStrategy(fs afero.Fs, store *config.Store) *OAuthStrategy {
	return &OAuthStrategy{
		profile:    linearOAuthProfile,
		graphqlURL: linearGraphQLURL,
		fs:         fs,
		store:      store,
		getenv:     os.Getenv,
		runFlow:    auth.RunFlowWith,
	}
}

// Add captures the OAuth app client credentials — --client-id, then
// LINEAR_CLIENT_ID (likewise for the PKCE-optional secret) — runs the
// browser flow with PKCE against the pinned endpoints, resolves the
// account identity through the viewer query, and persists the account
// under the linear provider with its token and client credentials.
func (s *OAuthStrategy) Add(_ context.Context, fs afero.Fs, store *config.Store, opts auth.AddOptions) (*config.Account, error) {
	clientID := strings.TrimSpace(opts.ClientID)
	if clientID == "" {
		clientID = strings.TrimSpace(s.getenv(envVarClientID))
	}
	if clientID == "" {
		return nil, fmt.Errorf("no OAuth client ID: pass --client-id or set %s", envVarClientID)
	}
	clientSecret := strings.TrimSpace(opts.ClientSecret)
	if clientSecret == "" {
		clientSecret = strings.TrimSpace(s.getenv(envVarClientSecret))
	}
	if clientSecret != "" {
		// Mint point: register before any output path exists.
		auth.RegisterSecret(clientSecret)
	}

	scopes := opts.Scopes
	if len(scopes) == 0 {
		scopes = append([]string{}, s.profile.DefaultScopes...)
	}

	// Attach identity resolution: Linear has no userinfo GET, so the
	// freshly exchanged token queries the GraphQL viewer. The full viewer
	// is captured for the account's Identity field.
	var who viewer
	profile := s.profile
	profile.IdentityResolver = func(ctx context.Context, tok *oauth2.Token) (string, error) {
		v, err := queryViewer(ctx, s.graphqlURL, tok.AccessToken)
		if err != nil {
			return "", err
		}
		who = v
		return v.Email, nil
	}

	credentialsPath, cleanup, err := writeClientCredentials(fs, clientID, clientSecret)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	tok, email, err := s.runFlow(fs, credentialsPath, scopes, profile)
	if err != nil {
		return nil, err
	}
	// Mint point for this save path: the token's secrets must be scrubbed
	// from any later output (RunFlowWith already registers; repeat for
	// substitute flows).
	auth.RegisterSecret(tok.AccessToken)
	auth.RegisterSecret(tok.RefreshToken)

	payload, err := json.Marshal(oauthAuthPayload{ClientID: clientID, ClientSecret: clientSecret})
	if err != nil {
		return nil, fmt.Errorf("encoding auth payload: %w", err)
	}
	acct := &config.Account{
		Name:     opts.Name,
		Provider: ID,
		Email:    email,
		Scopes:   scopes,
		Token:    tok,
		Identity: who.identity(),
		Auth:     payload,
	}
	if err := store.Save(acct); err != nil {
		return nil, err
	}
	// Save deduplicates by email within the provider, possibly under an
	// existing name; acct.Name reflects the canonical name.
	return store.GetProvider(ID, acct.Name)
}

// Client builds an *http.Client whose transport sources Bearer tokens from
// the account's stored token, refreshing against the profile's pinned
// token endpoint and persisting refreshes back to the linear account file.
func (s *OAuthStrategy) Client(ctx context.Context, acct *config.Account) (*http.Client, error) {
	if acct == nil {
		return nil, errors.New("no account")
	}
	var payload oauthAuthPayload
	if err := json.Unmarshal(acct.Auth, &payload); err != nil {
		return nil, fmt.Errorf("parsing account %q auth: %w", acct.Name, err)
	}
	if payload.ClientID == "" {
		return nil, fmt.Errorf("account %q holds no OAuth client ID", acct.Name)
	}
	// Read point: secrets restored from disk must be scrubbed from output.
	if acct.Token != nil {
		auth.RegisterSecret(acct.Token.AccessToken)
		auth.RegisterSecret(acct.Token.RefreshToken)
	}
	if payload.ClientSecret != "" {
		auth.RegisterSecret(payload.ClientSecret)
	}
	credentialsPath, cleanup, err := writeClientCredentials(s.fs, payload.ClientID, payload.ClientSecret)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	ts, err := auth.TokenSourceForProvider(s.fs, s.store, credentialsPath, ID, acct.Name, s.profile)
	if err != nil {
		return nil, err
	}
	return oauth2.NewClient(ctx, ts), nil
}

// writeClientCredentials renders the installed-app credentials document
// the generalized flow/token machinery parses, carrying ONLY the client
// credentials — the endpoints come from the pinned profile, never from any
// file. The file lands in the fs temp dir (0600 via the store's parent
// dirs are not involved, so tighten explicitly) and the returned cleanup
// removes it.
func writeClientCredentials(fs afero.Fs, clientID, clientSecret string) (path string, cleanup func(), err error) {
	data, err := json.Marshal(map[string]any{
		"installed": map[string]any{
			"client_id":     clientID,
			"client_secret": clientSecret,
			"redirect_uris": []string{"http://localhost"},
		},
	})
	if err != nil {
		return "", nil, fmt.Errorf("encoding client credentials: %w", err)
	}
	f, err := afero.TempFile(fs, "", "linear-oauth-*.json")
	if err != nil {
		return "", nil, fmt.Errorf("creating client credentials file: %w", err)
	}
	path = f.Name()
	cleanup = func() { _ = fs.Remove(path) }
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		cleanup()
		return "", nil, fmt.Errorf("writing client credentials: %w", err)
	}
	if err := f.Close(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("writing client credentials: %w", err)
	}
	if err := fs.Chmod(path, 0o600); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("securing client credentials: %w", err)
	}
	return path, cleanup, nil
}

// queryViewer resolves the token owner's identity through Linear's GraphQL
// viewer query — Linear's identity endpoint.
func queryViewer(ctx context.Context, url, accessToken string) (viewer, error) {
	body, err := json.Marshal(map[string]string{
		"query": "query { viewer { id name email } }",
	})
	if err != nil {
		return viewer{}, fmt.Errorf("encoding viewer query: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return viewer{}, fmt.Errorf("building viewer request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return viewer{}, fmt.Errorf("calling viewer endpoint: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return viewer{}, fmt.Errorf("viewer endpoint returned %s", resp.Status)
	}
	var out struct {
		Data struct {
			Viewer viewer `json:"viewer"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return viewer{}, fmt.Errorf("decoding viewer response: %w", err)
	}
	if out.Data.Viewer.Email == "" {
		return viewer{}, errors.New("viewer response carried no email")
	}
	return out.Data.Viewer, nil
}
