package slides

import (
	"context"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/auth"
	"github.com/oskarhane/google-cli/internal/subcommands/drive/service"
)

// dial is the service seam handed to every slides leaf: auth.DialAccount
// resolves the acting account and its token, the scope guard rejects narrowed
// grants before any API call, and service.New binds the full drive surface
// (including the slides client) to it. Leaves narrow via service.As, so
// tests substitute a fake-returning func.
func dial(ctx context.Context, cfg *app.Config) (service.DriveService, error) {
	acct, ts, err := auth.DialAccount(cfg)
	if err != nil {
		return nil, err
	}
	if err := auth.RequireScopes(acct, auth.ScopesSlides); err != nil {
		return nil, err
	}
	return service.New(ctx, ts)
}
