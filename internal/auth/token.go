package auth

import (
	"context"
	"fmt"

	"github.com/oskarhane/google-cli/internal/config"
	"github.com/spf13/afero"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// credentialsConfig parses installed-app credentials JSON into an OAuth
// config. Seam for tests, which point the endpoints at a local server.
var credentialsConfig = func(data []byte, scopes ...string) (*oauth2.Config, error) {
	return google.ConfigFromJSON(data, scopes...)
}

// TokenSource returns an oauth2.TokenSource for the named stored account.
// Valid tokens are reused; expired tokens are refreshed against Google, and
// a refreshed token is persisted back to the account file (0600).
func TokenSource(fs afero.Fs, store *config.Store, credentialsPath, name string) (oauth2.TokenSource, error) {
	acct, err := store.Get(name)
	if err != nil {
		return nil, err
	}
	data, err := afero.ReadFile(fs, credentialsPath)
	if err != nil {
		return nil, fmt.Errorf("reading credentials %s: %w", credentialsPath, err)
	}
	conf, err := credentialsConfig(data, acct.Scopes...)
	if err != nil {
		return nil, fmt.Errorf("parsing credentials %s: %w", credentialsPath, err)
	}
	return oauth2.ReuseTokenSource(acct.Token, &persistingSource{
		store: store,
		name:  name,
		conf:  conf,
		last:  acct.Token,
	}), nil
}

// persistingSource refreshes an account's token and writes the new token
// back to the account file. Google does not always re-issue a refresh token,
// so the previous one is preserved when the response omits it.
type persistingSource struct {
	store *config.Store
	name  string
	conf  *oauth2.Config
	last  *oauth2.Token
}

func (p *persistingSource) Token() (*oauth2.Token, error) {
	tok, err := p.conf.TokenSource(context.Background(), p.last).Token()
	if err != nil {
		return nil, fmt.Errorf("refreshing token for account %q: %w", p.name, err)
	}
	if tok.RefreshToken == "" && p.last != nil {
		tok.RefreshToken = p.last.RefreshToken
	}
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
	acct, err := p.store.Get(p.name)
	if err != nil {
		return err
	}
	acct.Token = tok
	if err := p.store.Save(acct); err != nil {
		return fmt.Errorf("persisting refreshed token for account %q: %w", p.name, err)
	}
	return nil
}
