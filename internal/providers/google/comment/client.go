package comment

import (
	"context"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/auth"
	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
)

// dial is the service seam handed to every comment leaf: auth.DialAccount
// resolves the acting account and its token, the scope guard rejects narrowed
// grants before any API call, and drive service.New binds the Drive surface
// to the token. Leaves call it from RunE, so tests substitute fakes.
//
// The guard accepts either drive grant, like the drive tree's dial: the Drive
// comments endpoints do not accept the documents/presentations scopes, so a
// comment dial under docs or slides still needs a drive grant.
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
