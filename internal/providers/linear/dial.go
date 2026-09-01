package linear

import (
	"context"
	"fmt"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/config"
	"github.com/oskarhane/google-cli/internal/providers/linear/service"
)

// resolveAccount resolves the linear account a command acts as: the
// --account flag value when given, else the provider's default account.
// Errors name linear so a multi-provider CLI never sends the user hunting
// in the wrong provider's accounts.
func resolveAccount(cfg *app.Config) (*config.Account, error) {
	store, err := config.NewStore(cfg.Fs, "")
	if err != nil {
		return nil, err
	}
	name := cfg.Account
	if name == "" {
		name, err = store.DefaultAccountFor(ID)
		if err != nil {
			return nil, err
		}
	}
	if name == "" {
		accounts, err := store.ListProvider(ID)
		if err != nil {
			return nil, err
		}
		if len(accounts) == 0 {
			return nil, fmt.Errorf("no linear accounts configured; run `linear account add`")
		}
		return nil, fmt.Errorf("no default linear account; pass --account or run `linear account use <name>`")
	}
	acct, err := store.GetProvider(ID, name)
	if err != nil {
		return nil, fmt.Errorf("resolving linear account %q: %w", name, err)
	}
	return acct, nil
}

// dial is the service seam handed to every linear subtree: it resolves the
// acting account, builds an authenticated HTTP client from the provider's
// API-key strategy, and binds a service.Service to it. Leaves call it from
// RunE, so tests substitute a fake-returning func.
func dial(ctx context.Context, cfg *app.Config) (*service.Service, error) {
	acct, err := resolveAccount(cfg)
	if err != nil {
		return nil, err
	}
	client, err := newStrategy().Client(ctx, acct)
	if err != nil {
		return nil, err
	}
	return service.New(client), nil
}
