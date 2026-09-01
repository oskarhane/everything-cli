package docs

import (
	"context"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/auth"
	"github.com/oskarhane/google-cli/internal/subcommands/drive/service"
)

// dial is the service seam handed to every docs leaf: auth.Dial resolves the
// acting account and its token; drive service.New binds the whole Workspace
// surface (drive, docs, sheets, slides clients share one transport) to it.
// Leaves call it from RunE, so tests substitute a fake-returning func.
func dial(ctx context.Context, cfg *app.Config) (service.DriveService, error) {
	ts, _, err := auth.Dial(cfg)
	if err != nil {
		return nil, err
	}
	return service.New(ctx, ts)
}
