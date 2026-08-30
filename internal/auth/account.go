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
		return "", fmt.Errorf("no Google accounts configured; run `google-cli account add`")
	}
	return "", fmt.Errorf("no default account set; run `google-cli account use <name>` or pass --account")
}

// Dial owns the whole auth chain every API-backed command shares: it opens
// the account store, resolves the acting account (cfg.Account first, then
// the store default), locates the OAuth credentials file, and returns a
// TokenSource for that account together with its name. Command packages
// wrap the result in their service constructor; no account-selection policy
// remains in command packages.
func Dial(cfg *app.Config) (oauth2.TokenSource, string, error) {
	store, err := config.NewStore(cfg.Fs, "")
	if err != nil {
		return nil, "", err
	}
	account, err := ResolveAccount(cfg, store)
	if err != nil {
		return nil, "", err
	}
	credentials, err := ResolveCredentials(cfg.Fs, cfg.Credentials, store.Dir())
	if err != nil {
		return nil, "", err
	}
	ts, err := TokenSource(cfg.Fs, store, credentials, account)
	if err != nil {
		return nil, "", fmt.Errorf("account %q: %w", account, err)
	}
	return ts, account, nil
}
