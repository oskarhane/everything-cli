package linear

import (
	"context"
	"net/http"

	"github.com/spf13/afero"

	"github.com/oskarhane/everything-cli/internal/auth"
	"github.com/oskarhane/everything-cli/internal/auth/apikey"
	"github.com/oskarhane/everything-cli/internal/config"
)

// strategy is Linear's composite auth.Strategy: the personal API key is
// the default onboarding path, OAuth (authorization-code + PKCE) the
// opt-in --oauth path. Add dispatches on AddOptions.UseOAuth; Client
// dispatches on the stored account's shape (an OAuth account carries a
// token, an API-key account only the key payload).
type strategy struct {
	apiKey *apikey.Strategy
	oauth  *OAuthStrategy
}

// Compile-time proof that strategy satisfies the auth seam.
var _ auth.Strategy = (*strategy)(nil)

// newStrategy builds Linear's composite strategy: the API-key strategy
// (raw key in the Authorization header, no Bearer prefix, captured from
// --api-key, then LINEAR_API_KEY, then a hidden prompt) plus the OAuth
// strategy backed by the store for token refresh. Both register their
// secrets for redaction at capture/read, so they never reach output.
func newStrategy(store *config.Store) *strategy {
	return &strategy{
		apiKey: apikey.Must(apikey.Config{
			Provider:     ID,
			HeaderName:   "Authorization",
			HeaderFormat: "%s",
			EnvVar:       envVarAPIKey,
		}),
		oauth: newOAuthStrategy(store),
	}
}

// Add onboards through the OAuth strategy when opts.UseOAuth is set
// (`linear account add --oauth`), else the default API-key capture.
func (s *strategy) Add(ctx context.Context, fs afero.Fs, store *config.Store, opts auth.AddOptions) (*config.Account, error) {
	if opts.UseOAuth {
		return s.oauth.Add(ctx, fs, store, opts)
	}
	return s.apiKey.Add(ctx, fs, store, opts)
}

// Client builds the authenticated client for the account's auth variant:
// a refreshing Bearer-token client for OAuth accounts (those carry a
// token), the static API-key header client otherwise.
func (s *strategy) Client(ctx context.Context, acct *config.Account) (*http.Client, error) {
	if acct != nil && acct.Token != nil {
		return s.oauth.Client(ctx, acct)
	}
	return s.apiKey.Client(ctx, acct)
}
