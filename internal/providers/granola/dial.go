package granola

import (
	"context"
	"fmt"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/config"
)

// resolveAccount picks the acting granola account: the --account flag value
// when given, else the provider's default account. Every error names the
// provider so a multi-provider user knows which account store to fix.
func resolveAccount(store *config.Store, name string) (*config.Account, error) {
	if name != "" {
		acct, err := store.GetProvider(providerID, name)
		if err != nil {
			return nil, fmt.Errorf("resolving granola account %q: %w", name, err)
		}
		return acct, nil
	}
	def, err := store.DefaultAccountFor(providerID)
	if err != nil {
		return nil, err
	}
	if def == "" {
		return nil, fmt.Errorf("no granola account configured: add one with `granola account add <name>` or pass --account")
	}
	acct, err := store.GetProvider(providerID, def)
	if err != nil {
		return nil, fmt.Errorf("resolving default granola account %q: %w", def, err)
	}
	return acct, nil
}

// dialNotes is the service seam handed to the note leaves: it resolves the
// acting account, builds the authenticated client from the API-key
// strategy, and binds the HTTP service to the public API base URL. Leaves
// call it from RunE, so tests substitute an httptest-backed service.
var dialNotes = func(ctx context.Context, cfg *app.Config) (NoteService, error) {
	store, err := config.NewStore(cfg.Fs, "")
	if err != nil {
		return nil, err
	}
	acct, err := resolveAccount(store, cfg.Account)
	if err != nil {
		return nil, err
	}
	client, err := strategy.Client(ctx, acct)
	if err != nil {
		return nil, err
	}
	return newHTTPService(client, defaultBaseURL), nil
}
