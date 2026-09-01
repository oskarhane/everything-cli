package auth

import (
	"fmt"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/config"
	"golang.org/x/oauth2"
)

// ResolveAccount returns the account to act as: the --account flag value,
// else the store's default account. It errors with actionable messages when
// no account can be picked. Every API-backed command resolves its account
// through here, so the guidance texts live in exactly one place.
func ResolveAccount(cfg *app.Config, store *config.Store) (string, error) {
	if cfg.Account != "" {
		return cfg.Account, nil
	}
	def, err := store.DefaultAccount()
	if err != nil {
		return "", err
	}
	if def != "" {
		return def, nil
	}
	accounts, err := store.List()
	if err != nil {
		return "", err
	}
	if len(accounts) == 0 {
		return "", fmt.Errorf("no Google accounts configured; run `google-cli google account add`")
	}
	return "", fmt.Errorf("no default account set; run `google-cli google account use <name>` or pass --account")
}

// DialAccount is Dial plus the resolved account record: trees that enforce
// a scope guardrail need the account's granted scopes, which the name alone
// does not carry. Same chain, one extra store read.
func DialAccount(cfg *app.Config) (*config.Account, oauth2.TokenSource, error) {
	store, err := config.NewStore(cfg.Fs, "")
	if err != nil {
		return nil, nil, err
	}
	account, err := ResolveAccount(cfg, store)
	if err != nil {
		return nil, nil, err
	}
	credentials, err := ResolveCredentials(cfg.Fs, cfg.Credentials, store.Dir())
	if err != nil {
		return nil, nil, err
	}
	acct, err := store.Get(account)
	if err != nil {
		return nil, nil, err
	}
	ts, err := TokenSource(cfg.Fs, store, credentials, account)
	if err != nil {
		return nil, nil, fmt.Errorf("account %q: %w", account, err)
	}
	return acct, ts, nil
}

// Dial owns the whole auth chain every API-backed command shares: it opens
// the account store, resolves the acting account (cfg.Account first, then
// the store default), locates the OAuth credentials file, and returns a
// TokenSource for that account together with its name. Command packages
// wrap the result in their service constructor; no account-selection policy
// remains in command packages.
func Dial(cfg *app.Config) (oauth2.TokenSource, string, error) {
	acct, ts, err := DialAccount(cfg)
	if err != nil {
		return nil, "", err
	}
	return ts, acct.Name, nil
}
