package auth

import (
	"context"
	"errors"
	"net/http"

	"github.com/oskarhane/everything-cli/internal/config"
	"github.com/spf13/afero"
	"golang.org/x/oauth2"
)

// Strategy is the per-provider authentication seam: how a provider onboards
// a new account and how an authenticated HTTP client is built for a stored
// account. Providers implement it (or reuse OAuthStrategy) so the rest of
// the CLI never branches on auth mechanics.
type Strategy interface {
	// Add runs the onboarding for a new account — interactive or not —
	// and persists the result in store, returning the saved record. The
	// returned name may differ from opts.Name when the store deduplicates
	// an existing identity.
	Add(ctx context.Context, fs afero.Fs, store *config.Store, opts AddOptions) (*config.Account, error)
	// Client returns an authenticated HTTP client for the account,
	// refreshing the underlying credential as needed. *http.Client (not
	// oauth2.TokenSource) is the seam so non-OAuth strategies (API key,
	// bearer token) can implement it with a plain RoundTripper.
	Client(ctx context.Context, acct *config.Account) (*http.Client, error)
}

// AddOptions carries the onboarding inputs a Strategy may need. Fields not
// relevant to a strategy are ignored by it.
type AddOptions struct {
	// Name is the requested account name.
	Name string
	// Credentials carries the OAuth app's client credentials (OAuth
	// strategies only), already resolved from the provider's source — a
	// credentials file parsed once (Google) or flags/env (Linear).
	Credentials ClientCredentials
	// Scopes is the requested OAuth scope set; empty means the provider's
	// default scopes.
	Scopes []string
	// APIKey is a pre-captured API key (API-key strategies only), e.g.
	// from a --api-key flag; empty means the strategy captures it from
	// its env var or a hidden prompt.
	APIKey string
	// UseOAuth selects the OAuth path on providers offering more than one
	// auth strategy (e.g. `linear account add --oauth`); single-strategy
	// providers ignore it.
	UseOAuth bool
	// ClientID is the OAuth app's client ID for providers whose OAuth
	// needs no credentials file (captured from a --client-id flag or env
	// var); empty means the strategy resolves it from its env var.
	ClientID string
	// ClientSecret is the OAuth app's client secret. Optional for public
	// PKCE clients (Linear makes it optional when PKCE is used).
	ClientSecret string
}

// OAuthStrategy is the Strategy for installed-app OAuth2 providers. It
// composes the generalized flow (RunFlowWith), account persistence
// (SaveAccount) and the refreshing token source (TokenSourceWith) against
// one pinned OAuthProfile, so adding an OAuth provider is configuration,
// not new machinery.
type OAuthStrategy struct {
	profile OAuthProfile
	store   *config.Store
	creds   ClientCredentials
}

// Compile-time proof that OAuthStrategy satisfies the seam.
var _ Strategy = (*OAuthStrategy)(nil)

// NewOAuthStrategy returns a Strategy for profile. store and creds back
// Client's token refresh and persistence; Add uses the fs/store it is
// handed per call.
func NewOAuthStrategy(profile OAuthProfile, store *config.Store, creds ClientCredentials) *OAuthStrategy {
	return &OAuthStrategy{
		profile: profile,
		store:   store,
		creds:   creds,
	}
}

// Add runs the installed-app OAuth flow for the strategy's profile and
// saves the resulting account, exactly like `account add` does today for
// Google. Empty opts.Scopes falls back to the profile's default scopes;
// empty opts.Credentials falls back to the strategy's credentials.
func (s *OAuthStrategy) Add(_ context.Context, _ afero.Fs, store *config.Store, opts AddOptions) (*config.Account, error) {
	scopes := opts.Scopes
	if len(scopes) == 0 {
		scopes = s.profile.DefaultScopes
	}
	creds := opts.Credentials
	if creds.ID == "" {
		creds = s.creds
	}
	if creds.ID == "" {
		return nil, errors.New("no OAuth client credentials")
	}
	tok, email, err := RunFlowWith(creds, scopes, s.profile)
	if err != nil {
		return nil, err
	}
	saved, err := SaveAccount(store, opts.Name, email, scopes, tok)
	if err != nil {
		return nil, err
	}
	return store.Get(saved)
}

// Client builds an *http.Client whose transport sources tokens from the
// account's stored token, refreshing against the profile's pinned token
// endpoint and persisting refreshes back to the account file.
func (s *OAuthStrategy) Client(ctx context.Context, acct *config.Account) (*http.Client, error) {
	if acct == nil {
		return nil, errors.New("no account")
	}
	ts, err := TokenSourceWith(s.store, s.creds, acct.Name, s.profile)
	if err != nil {
		return nil, err
	}
	return oauth2.NewClient(ctx, ts), nil
}
