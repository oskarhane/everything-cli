package calendar

import (
	"context"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/auth"
	"github.com/oskarhane/everything-cli/internal/providers/google/calendar/service"
)

// dial is the service seam handed to every calendar subtree: auth.Dial
// resolves the acting account and its token; service.New binds a
// CalendarService to it. Leaves call it from RunE, so tests substitute a
// fake-returning func instead.
func dial(ctx context.Context, cfg *app.Config) (service.CalendarService, error) {
	ts, _, err := auth.Dial(cfg)
	if err != nil {
		return nil, err
	}
	return service.New(ctx, ts)
}
