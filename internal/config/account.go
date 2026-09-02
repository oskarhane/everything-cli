package config

import (
	"encoding/json"

	"golang.org/x/oauth2"
)

// ProviderGoogle is the provider ID for Google accounts. Accounts written
// before providers existed (flat accounts/<name>.json files with no
// provider field) load as Google accounts.
const ProviderGoogle = "google"

// Account is a persisted provider account: a name unique within its
// provider, the provider ID, and the provider-specific identity and auth
// material. Disk format uses snake_case JSON keys; Token reuses
// oauth2.Token's wire tags (access_token, refresh_token, token_type,
// expiry).
//
// The Google fields (Email, Scopes, Token) stay top level so legacy account
// files load transparently and existing callers (e.g. auth.DialAccount)
// keep working; Identity and Auth are the additive generic shape for
// providers whose auth is not a Google OAuth token (API keys, bearer
// tokens), opaque to the store.
type Account struct {
	Name     string            `json:"name"`
	Provider string            `json:"provider,omitempty"`
	Email    string            `json:"email"`
	Scopes   []string          `json:"scopes"`
	Token    *oauth2.Token     `json:"token"`
	Identity map[string]string `json:"identity,omitempty"`
	Auth     json.RawMessage   `json:"auth,omitempty"`
}
