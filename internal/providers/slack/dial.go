package slack

import (
	"context"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/auth"
)

// dialSlack is the service seam every slack resource leaf consumes: it
// resolves the acting account through the canonical provider-scoped resolver,
// builds the authenticated client from the API-key strategy, and binds the
// HTTP service to the Web API base URL. Leaves call it from RunE, so tests
// substitute an httptest-backed service.
var dialSlack = func(ctx context.Context, cfg *app.Config) (*httpService, error) {
	store, err := cfg.Store()
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
