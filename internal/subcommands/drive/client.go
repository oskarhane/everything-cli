package drive

import (
	"context"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/auth"
	"github.com/oskarhane/google-cli/internal/subcommands/drive/service"
)

// dial is the service seam handed to every drive subtree: auth.DialAccount
// resolves the acting account and its token, the scope guard rejects narrowed
// grants before any API call, and service.New binds a DriveService to the
// token. Leaves call it from RunE, so tests substitute a fake-returning func.
//
// The guard accepts either drive grant: the full drive scope, or the minimal
// drive.file profile (app-created files only) for accounts that opt out of
// account-wide sharing power. Subtrees that need the full grant — the sharing
// leaves — add their own guard on top.
func dial(ctx context.Context, cfg *app.Config) (service.DriveService, error) {
	acct, ts, err := auth.DialAccount(cfg)
	if err != nil {
		return nil, err
	}
	if err := auth.RequireAnyScopes(acct, auth.ScopesDriveDial); err != nil {
		return nil, err
	}
	return service.New(ctx, ts)
}
