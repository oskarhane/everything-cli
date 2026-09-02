package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/oskarhane/everything-cli/internal/config"
	"github.com/spf13/afero"
	"golang.org/x/oauth2"
)

// refreshTimeout bounds a token refresh against Google's token endpoint so
// a hung endpoint cannot stall a non-interactive command indefinitely.
// Var so tests can shrink it.
var refreshTimeout = 60 * time.Second

// oauthConfigFor builds the OAuth2 config for a flow or token refresh:
// the client credentials are taken directly (never from a file) and the
// endpoints are pinned to the profile's. Seam for tests, which point the
// endpoints at a local server.
var oauthConfigFor = func(profile OAuthProfile, creds ClientCredentials, scopes ...string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     creds.ID,
		ClientSecret: creds.Secret,
		Endpoint:     profile.Endpoint,
		Scopes:       scopes,
	}
}

// TokenSource returns an oauth2.TokenSource for the named stored account.
// Valid tokens are reused; expired tokens are refreshed against Google, and
// a refreshed token is persisted back to the account file (0600).
func TokenSource(fs afero.Fs, store *config.Store, credentialsPath, name string) (oauth2.TokenSource, error) {
	creds, err := ReadClientCredentials(fs, credentialsPath)
	if err != nil {
		return nil, err
	}
	return TokenSourceWith(store, creds, name, GoogleOAuth)
}

// TokenSourceWith is TokenSource generalized to any OAuth profile and any
// client credentials: the refresh targets the profile's pinned token
// endpoint, taken from the profile rather than from any file.
func TokenSourceWith(store *config.Store, creds ClientCredentials, name string, profile OAuthProfile) (oauth2.TokenSource, error) {
	return TokenSourceForProvider(store, creds, config.ProviderGoogle, name, profile)
}

// TokenSourceForProvider is TokenSourceWith scoped to a provider's account
// directory: the account is read from (and refreshes persisted back to)
// accounts/<provider>/<name>.json. Non-Google OAuth providers (Linear)
// onboard through it so their token cache refreshes exactly like Google's.
func TokenSourceForProvider(store *config.Store, creds ClientCredentials, provider, name string, profile OAuthProfile) (oauth2.TokenSource, error) {
	acct, err := store.GetProvider(provider, name)
	if err != nil {
		return nil, err
	}
	// Fail closed on identity mismatch: an account file whose "name" field
	// disagrees with the name it was stored under may be corrupted or copied
	// from another account.
	if acct.Name != name {
		return nil, fmt.Errorf(
			"account file %q contains name %q — refusing; the record may be corrupted or copied from another account",
			name, acct.Name)
	}
	// Read point: register the stored token's secrets for redaction.
	registerTokenSecrets(acct.Token)
	conf := oauthConfigFor(profile, creds, acct.Scopes...)
	return oauth2.ReuseTokenSource(acct.Token, &persistingSource{
		store:    store,
		provider: provider,
		name:     name,
		conf:     conf,
		last:     acct.Token,
	}), nil
}

// persistingSource refreshes an account's token and writes the new token
// back to the account file. Google does not always re-issue a refresh token,
// so the previous one is preserved when the response omits it.
type persistingSource struct {
	store    *config.Store
	provider string
	name     string
	conf     *oauth2.Config
	last     *oauth2.Token
}

func (p *persistingSource) Token() (*oauth2.Token, error) {
	ctx, cancel := context.WithTimeout(context.Background(), refreshTimeout)
	defer cancel()
	tok, err := p.conf.TokenSource(ctx, p.last).Token()
	if err != nil {
		return nil, fmt.Errorf("refreshing token for account %q: %w", p.name, err)
	}
	if tok.RefreshToken == "" && p.last != nil {
		tok.RefreshToken = p.last.RefreshToken
	}
	// Mint point: register the refreshed token's secrets for redaction.
	registerTokenSecrets(tok)
	if p.changed(tok) {
		if err := p.persist(tok); err != nil {
			return nil, err
		}
	}
	p.last = tok
	return tok, nil
}

// changed reports whether tok differs from the persisted one.
func (p *persistingSource) changed(tok *oauth2.Token) bool {
	if p.last == nil {
		return true
	}
	return tok.AccessToken != p.last.AccessToken ||
		tok.RefreshToken != p.last.RefreshToken ||
		!tok.Expiry.Equal(p.last.Expiry)
}

func (p *persistingSource) persist(tok *oauth2.Token) error {
	acct, err := p.store.GetProvider(p.provider, p.name)
	if err != nil {
		return err
	}
	acct.Token = tok
	if err := p.store.Save(acct); err != nil {
		return fmt.Errorf("persisting refreshed token for account %q: %w", p.name, err)
	}
	return nil
}
