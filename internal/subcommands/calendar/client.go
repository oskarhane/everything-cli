package calendar

import (
	"context"
	"fmt"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/auth"
	"github.com/oskarhane/google-cli/internal/config"
	"github.com/oskarhane/google-cli/internal/subcommands/calendar/service"
)

// dial is the service seam handed to every calendar subtree: it resolves the
// acting account, loads its stored token, and returns a CalendarService.
// Leaves call it from RunE, so tests substitute a fake-returning func
// instead.
func dial(ctx context.Context, cfg *app.Config) (service.CalendarService, error) {
	store, err := config.NewStore(cfg.Fs, "")
	if err != nil {
		return nil, err
	}
	account, err := resolveAccount(cfg, store)
	if err != nil {
		return nil, err
	}
	credentials, err := auth.ResolveCredentials(cfg.Fs, cfg.Credentials, store.Dir())
	if err != nil {
		return nil, err
	}
	ts, err := auth.TokenSource(cfg.Fs, store, credentials, account)
	if err != nil {
		return nil, fmt.Errorf("account %q: %w", account, err)
	}
	return service.New(ctx, ts)
}

// resolveAccount returns the account to act as: the --account flag value,
// else the store's default account. It errors with actionable messages when
// no account can be picked.
func resolveAccount(cfg *app.Config, store *config.Store) (string, error) {
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
	return "", fmt.Errorf("no default account set; run `google-cli account default <name>` or pass --account")
}
