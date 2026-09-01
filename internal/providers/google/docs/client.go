package docs

import (
	"context"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/auth"
	"github.com/oskarhane/google-cli/internal/subcommands/drive/service"
)

// dial is the service seam handed to every docs leaf: auth.DialAccount
// resolves the acting account and its token, the scope guard rejects narrowed
// grants before any API call, and drive service.New binds the whole Workspace
// surface (drive, docs, sheets, slides clients share one transport) to it.
// Leaves call it from RunE, so tests substitute a fake-returning func.
func dial(ctx context.Context, cfg *app.Config) (service.DriveService, error) {
	acct, ts, err := auth.DialAccount(cfg)
	if err != nil {
		return nil, err
	}
	if err := auth.RequireScopes(acct, auth.ScopesDocs); err != nil {
		return nil, err
	}
	return service.New(ctx, ts)
}
