package granola

import (
	"context"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/auth"
	"github.com/oskarhane/everything-cli/internal/config"
)

// dialNotes is the service seam handed to the note leaves: it resolves the
// acting account through the canonical provider-scoped resolver, builds the
// authenticated client from the API-key strategy, and binds the HTTP
// service to the public API base URL. Leaves call it from RunE, so tests
// substitute an httptest-backed service.
var dialNotes = func(ctx context.Context, cfg *app.Config) (NoteService, error) {
	store, err := config.NewStore(cfg.Fs, "")
	if err != nil {
		return nil, err
	}
	acct, err := auth.ResolveAccountFor(cfg, store, providerID)
	if err != nil {
		return nil, err
	}
	client, err := strategy.Client(ctx, acct)
	if err != nil {
		return nil, err
	}
	return newHTTPService(client, defaultBaseURL), nil
}
