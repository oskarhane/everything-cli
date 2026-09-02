package auth

import (
	"fmt"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/config"
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
		return "", fmt.Errorf("no Google accounts configured; run `everything-cli google account add`")
	}
	return "", fmt.Errorf("no default account set; run `everything-cli google account use <name>` or pass --account")
}

// ResolveAccountFor is the canonical provider-scoped account resolver: it
// returns the account a provider's command acts as — the --account flag
// value when given, else the provider's default account — as the full
// account record. Every error names the provider so a multi-provider CLI
// never sends the user hunting in the wrong provider's accounts. Google's
// OAuth trees keep ResolveAccount above (legacy unscoped texts); API-key
// providers resolve through here.
func ResolveAccountFor(cfg *app.Config, store *config.Store, providerID string) (*config.Account, error) {
	name := cfg.Account
	if name == "" {
		def, err := store.DefaultAccountFor(providerID)
		if err != nil {
			return nil, err
		}
		name = def
	}
	if name == "" {
		accounts, err := store.ListProvider(providerID)
		if err != nil {
			return nil, err
		}
		if len(accounts) == 0 {
			return nil, fmt.Errorf("no %s accounts configured; run `everything-cli %s account add`", providerID, providerID)
		}
		return nil, fmt.Errorf("no default %s account set; run `everything-cli %s account use <name>` or pass --account", providerID, providerID)
	}
	acct, err := store.GetProvider(providerID, name)
	if err != nil {
		return nil, fmt.Errorf("resolving %s account %q: %w", providerID, name, err)
	}
	return acct, nil
}

// DialAccount is Dial plus the resolved account record: trees that enforce
// a scope guardrail need the account's granted scopes, which the name alone
// does not carry. Same chain, one extra store read.
func DialAccount(cfg *app.Config) (*config.Account, oauth2.TokenSource, error) {
	store, err := cfg.Store()
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
	creds, err := ReadClientCredentials(cfg.Fs, credentials)
	if err != nil {
		return nil, nil, err
	}
	acct, err := store.Get(account)
	if err != nil {
		return nil, nil, err
	}
	ts, err := TokenSourceWith(store, creds, account, GoogleOAuth)
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
