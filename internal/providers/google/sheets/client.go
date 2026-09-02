package sheets

import (
	"context"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/auth"
	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
)

// dial is the service seam handed to every sheets subtree: auth.DialAccount
// resolves the acting account and its token, the scope guard rejects narrowed
// grants before any API call, and service.New binds a DriveService (whose
// sheets methods ride the same authenticated client) to it. Leaves call it
// from RunE, so tests substitute a fake-returning func.
func dial(ctx context.Context, cfg *app.Config) (service.DriveService, error) {
	acct, ts, err := auth.DialAccount(cfg)
	if err != nil {
		return nil, err
	}
	if err := auth.RequireScopes(acct, auth.ScopesSheets); err != nil {
		return nil, err
	}
	return service.New(ctx, ts)
}
