package auth

import (
	"context"
	"errors"
	"net/http"

	"github.com/oskarhane/google-cli/internal/config"
	"github.com/spf13/afero"
	"golang.org/x/oauth2"
)

// Strategy is the per-provider authentication seam: how a provider onboards
// a new account, how an authenticated HTTP client is built for a stored
// account, and which account fields are secrets. Providers implement it
// (or reuse OAuthStrategy) so the rest of the CLI never branches on auth
// mechanics.
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
	// SecretFields names the account JSON fields whose values are secrets
	// to register for redaction at the mint/read point (AGENTS.md rule).
	SecretFields() []string
}

// AddOptions carries the onboarding inputs a Strategy may need. Fields not
// relevant to a strategy are ignored by it.
type AddOptions struct {
	// Name is the requested account name.
	Name string
	// CredentialsPath is the OAuth installed-app credentials JSON (OAuth
	// strategies only), already resolved from flag or config dir — never
	// the working directory.
	CredentialsPath string
	// Scopes is the requested OAuth scope set; empty means the provider's
	// default scopes.
	Scopes []string
}

// OAuthStrategy is the Strategy for installed-app OAuth2 providers. It
// composes the generalized flow (RunFlowWith), account persistence
// (SaveAccount) and the refreshing token source (TokenSourceWith) against
// one pinned OAuthProfile, so adding an OAuth provider is configuration,
// not new machinery.
type OAuthStrategy struct {
	profile         OAuthProfile
	fs              afero.Fs
	store           *config.Store
	credentialsPath string
}

// Compile-time proof that OAuthStrategy satisfies the seam.
var _ Strategy = (*OAuthStrategy)(nil)

// NewOAuthStrategy returns a Strategy for profile. fs, store and
// credentialsPath back Client's token refresh and persistence; Add uses the
// fs/store it is handed per call.
func NewOAuthStrategy(profile OAuthProfile, fs afero.Fs, store *config.Store, credentialsPath string) *OAuthStrategy {
	return &OAuthStrategy{
		profile:         profile,
		fs:              fs,
		store:           store,
		credentialsPath: credentialsPath,
	}
}

// Add runs the installed-app OAuth flow for the strategy's profile and
// saves the resulting account, exactly like `account add` does today for
// Google. Empty opts.Scopes falls back to the profile's default scopes.
func (s *OAuthStrategy) Add(_ context.Context, fs afero.Fs, store *config.Store, opts AddOptions) (*config.Account, error) {
	scopes := opts.Scopes
	if len(scopes) == 0 {
		scopes = s.profile.DefaultScopes
	}
	tok, email, err := RunFlowWith(fs, opts.CredentialsPath, scopes, s.profile)
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
	ts, err := TokenSourceWith(s.fs, s.store, s.credentialsPath, acct.Name, s.profile)
	if err != nil {
		return nil, err
	}
	return oauth2.NewClient(ctx, ts), nil
}

// SecretFields names the token fields of the account document: both token
// values are as sensitive as a Google refresh token and must never print.
func (s *OAuthStrategy) SecretFields() []string {
	return []string{"token.access_token", "token.refresh_token"}
}
