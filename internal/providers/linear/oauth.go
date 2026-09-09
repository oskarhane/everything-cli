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
	store      *config.Store
	getenv     func(string) string
	// runFlow is the flow seam; production is auth.RunFlowWith, tests a
	// hermetic flow against fake endpoints.
	runFlow func(creds auth.ClientCredentials, scopes []string, profile auth.OAuthProfile) (*oauth2.Token, string, error)
}

// Compile-time proof that OAuthStrategy satisfies the auth seam.
var _ auth.Strategy = (*OAuthStrategy)(nil)

// Compile-time proof that OAuthStrategy carries the re-authorization
// capability on top of the base Strategy seam.
var _ auth.Reauther = (*OAuthStrategy)(nil)

// newOAuthStrategy builds the production OAuth strategy. store backs
// Client's token refresh and persistence; Add uses the fs/store it is
// handed per call.
func newOAuthStrategy(store *config.Store) *OAuthStrategy {
	return &OAuthStrategy{
		profile:    linearOAuthProfile,
		graphqlURL: linearGraphQLURL,
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
func (s *OAuthStrategy) Add(_ context.Context, _ afero.Fs, store *config.Store, opts auth.AddOptions) (*config.Account, error) {
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

	profile, who := s.profileWithIdentityResolver()

	tok, email, err := s.runFlow(auth.ClientCredentials{ID: clientID, Secret: clientSecret}, scopes, profile)
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

// Reauth re-runs the browser OAuth flow for an existing OAuth account and
// replaces its stored token in place, reusing the client credentials the
// account's Auth payload carries — there is no credentials file to re-read,
// and re-auth must never lose the app credentials Client needs for refresh.
// Unlike Add, the account's identity is pinned: if the browser session
// authorizes a different email, nothing is saved and the caller is told to
// onboard that identity with `linear account add` instead — a re-auth must
// never silently create or corrupt an account.
//
// Scope resolution mirrors the account, not the defaults: opts.Scopes wins,
// then the account's currently granted scopes (so a deliberately narrowed
// grant survives re-authorization), then the profile's defaults for a
// stored account that carries no scopes at all.
func (s *OAuthStrategy) Reauth(_ context.Context, _ afero.Fs, store *config.Store, acct *config.Account, opts auth.ReauthOptions) (*config.Account, error) {
	payload, err := s.storedCredentials(acct)
	if err != nil {
		return nil, err
	}

	// The cascade (explicit opts, then the account's granted scopes, then
	// the profile's defaults) lives in auth.ResolveReauthScopes.
	scopes := auth.ResolveReauthScopes(opts.Scopes, acct.Scopes, s.profile.DefaultScopes)

	profile, who := s.profileWithIdentityResolver()

	tok, email, err := s.runFlow(auth.ClientCredentials{ID: payload.ClientID, Secret: payload.ClientSecret}, scopes, profile)
	if err != nil {
		return nil, err
	}
	// Mint point for this save path: the token's secrets must be scrubbed
	// from any later output (RunFlowWith already registers; repeat for
	// substitute flows).
	auth.RegisterSecret(tok.AccessToken)
	auth.RegisterSecret(tok.RefreshToken)

	if acct.Email != "" && email != acct.Email {
		return nil, fmt.Errorf("authorized identity %q does not match account %q (%q); re-run in the browser as the same account, or use \"linear account add\" to onboard a different identity", email, acct.Name, acct.Email)
	}

	// Persist under the same name, preserving the stored Auth payload: the
	// client credentials it holds are still the account's.
	updated := &config.Account{
		Name:     acct.Name,
		Provider: ID,
		Email:    email,
		Scopes:   scopes,
		Token:    tok,
		Identity: who.identity(),
		Auth:     acct.Auth,
	}
	if err := store.Save(updated); err != nil {
		return nil, err
	}
	return store.GetProvider(ID, acct.Name)
}

// storedCredentials decodes the account's OAuth app credentials payload
// and requires the client ID the flow and the refreshing token source
// both need. It is also the single read point for the account's stored
// secrets: the token's access/refresh values and the payload's client
// secret are registered for redaction here, so Client and Reauth share
// one scrubbing site instead of duplicating it.
func (s *OAuthStrategy) storedCredentials(acct *config.Account) (oauthAuthPayload, error) {
	var payload oauthAuthPayload
	if err := json.Unmarshal(acct.Auth, &payload); err != nil {
		return payload, fmt.Errorf("parsing account %q auth: %w", acct.Name, err)
	}
	if payload.ClientID == "" {
		return payload, fmt.Errorf("account %q holds no OAuth client ID", acct.Name)
	}
	// Read point: secrets restored from disk must be scrubbed from output.
	if acct.Token != nil {
		auth.RegisterSecret(acct.Token.AccessToken)
		auth.RegisterSecret(acct.Token.RefreshToken)
	}
	if payload.ClientSecret != "" {
		auth.RegisterSecret(payload.ClientSecret)
	}
	return payload, nil
}

// profileWithIdentityResolver returns a copy of the strategy profile with
// the GraphQL viewer identity resolver attached, plus a pointer to the
// viewer the resolver captures — Linear has no userinfo GET, so the
// freshly exchanged token queries the GraphQL viewer, and the full viewer
// (not just the email the resolver returns) feeds the account's Identity
// field. Add and Reauth share this attachment.
func (s *OAuthStrategy) profileWithIdentityResolver() (auth.OAuthProfile, *viewer) {
	who := &viewer{}
	profile := s.profile
	profile.IdentityResolver = func(ctx context.Context, tok *oauth2.Token) (string, error) {
		v, err := queryViewer(ctx, s.graphqlURL, tok.AccessToken)
		if err != nil {
			return "", err
		}
		*who = v
		return v.Email, nil
	}
	return profile, who
}

// Client builds an *http.Client whose transport sources Bearer tokens from
// the account's stored token, refreshing against the profile's pinned
// token endpoint and persisting refreshes back to the linear account file.
func (s *OAuthStrategy) Client(ctx context.Context, acct *config.Account) (*http.Client, error) {
	if acct == nil {
		return nil, errors.New("no account")
	}
	payload, err := s.storedCredentials(acct)
	if err != nil {
		return nil, err
	}
	creds := auth.ClientCredentials{ID: payload.ClientID, Secret: payload.ClientSecret}
	ts, err := auth.TokenSourceForProvider(s.store, creds, ID, acct.Name, s.profile)
	if err != nil {
		return nil, err
	}
	return oauth2.NewClient(ctx, ts), nil
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
