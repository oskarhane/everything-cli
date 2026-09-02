package linear

import (
	"context"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/auth"
	"github.com/oskarhane/everything-cli/internal/config"
	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
)

// dial is the service seam handed to every linear subtree: it opens the
// account store once, resolves the acting account through the canonical
// provider-scoped resolver, builds an authenticated HTTP client from the
// provider's API-key strategy, and binds a service.Service to it. Leaves
// call it from RunE, so tests substitute a fake-returning func.
func dial(ctx context.Context, cfg *app.Config) (*service.Service, error) {
	store, err := config.NewStore(cfg.Fs, "")
	if err != nil {
		return nil, err
	}
	acct, err := auth.ResolveAccountFor(cfg, store, ID)
	if err != nil {
		return nil, err
	}
	client, err := newStrategy(store).Client(ctx, acct)
	if err != nil {
		return nil, err
	}
	return service.New(client), nil
}
