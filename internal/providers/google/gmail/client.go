package gmail

import (
	"context"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/auth"
	"github.com/oskarhane/google-cli/internal/providers/google/gmail/service"
)

// dial is the service seam handed to every gmail subtree: auth.Dial resolves
// the acting account and its token; service.New binds a GmailService to it.
// Leaves call it from RunE, so tests substitute a fake-returning func.
func dial(ctx context.Context, cfg *app.Config) (service.GmailService, error) {
	ts, _, err := auth.Dial(cfg)
	if err != nil {
		return nil, err
	}
	return service.New(ctx, ts)
}
