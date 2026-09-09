package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/oskarhane/everything-cli/internal/config"
	"github.com/spf13/afero"
)

// Compile-time proof that OAuthStrategy carries the re-authorization
// capability on top of the base Strategy seam.
var _ Reauther = (*OAuthStrategy)(nil)

// Reauth re-runs the interactive OAuth flow for an existing account and
// updates its stored token in place. Unlike Add, the account's identity is
// pinned: if the browser session authorizes a different email, nothing is
// saved and the caller is told to onboard that identity with `account add`
// instead — a re-auth must never silently create or corrupt an account.
//
// Scope resolution mirrors the account, not the defaults: opts.Scopes wins,
// then the account's currently granted scopes (so a deliberately narrowed
// grant survives re-authorization), then the profile's defaults for a
// stored account that carries no scopes at all.
func (s *OAuthStrategy) Reauth(_ context.Context, _ afero.Fs, store *config.Store, acct *config.Account, opts ReauthOptions) (*config.Account, error) {
	scopes := opts.Scopes
	if len(scopes) == 0 {
		scopes = acct.Scopes
	}
	if len(scopes) == 0 {
		scopes = s.profile.DefaultScopes
	}
	if s.creds.ID == "" {
		return nil, errors.New("no OAuth client credentials")
	}
	tok, email, err := RunFlowWith(s.creds, scopes, s.profile)
	if err != nil {
		return nil, err
	}
	if acct.Email != "" && email != acct.Email {
		return nil, fmt.Errorf("authorized identity %q does not match account %q (%q); re-run in the browser as the same account, or use \"account add\" to onboard a different identity", email, acct.Name, acct.Email)
	}
	saved, err := SaveAccount(store, acct.Name, email, scopes, tok)
	if err != nil {
		return nil, err
	}
	return store.Get(saved)
}
