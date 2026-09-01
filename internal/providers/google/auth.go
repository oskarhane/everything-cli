// Package google wires Google as a provider of the CLI. This file is its
// auth adapter: Google's installed-app OAuth flow behind the auth.Strategy
// seam, so the account machinery never branches on auth mechanics.
package google

import (
	"github.com/spf13/afero"

	"github.com/oskarhane/everything-cli/internal/auth"
	"github.com/oskarhane/everything-cli/internal/config"
)

// secretFieldNames are the account JSON fields holding Google's OAuth secrets:
// token.access_token and token.refresh_token. Both token values are as
// sensitive as a refresh token and must never print; they are registered
// for redaction at the mint/read point (see internal/auth/redact.go).
var secretFieldNames = []string{"token.access_token", "token.refresh_token"}

// Strategy is Google's auth.Strategy. Add runs the existing installed-app
// browser OAuth flow (auth.RunFlowWith with the pinned GoogleOAuth profile)
// and persists the account through the provider-scoped store; Client
// returns a refreshing *http.Client over the stored token
// (auth.TokenSourceWith, persisted back on refresh).
type Strategy struct {
	*auth.OAuthStrategy
}

// Compile-time proof that Strategy satisfies the seam.
var _ auth.Strategy = (*Strategy)(nil)

// NewStrategy returns Google's auth strategy. fs, store and
// credentialsPath back Client's token refresh and persistence; Add uses the
// fs/store it is handed per call.
func NewStrategy(fs afero.Fs, store *config.Store, credentialsPath string) *Strategy {
	return &Strategy{auth.NewOAuthStrategy(auth.GoogleOAuth, fs, store, credentialsPath)}
}

// SecretFields names the secret-bearing fields of a Google account
// document: token.access_token and token.refresh_token.
func (s *Strategy) SecretFields() []string {
	return append([]string{}, secretFieldNames...)
}
