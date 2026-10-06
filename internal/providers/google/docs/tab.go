package docs

import (
	"context"

	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
)

// resolveTabID resolves a --tab key (tab ID or exact title) to a tab ID via
// the service's shared resolution. Leaves call it because the write methods
// forward the tab ID as-is and make no read to resolve a title; an empty key
// returns an empty tab ID without any service call, letting the API apply
// the operation to the first tab.
func resolveTabID(ctx context.Context, svc service.DocService, docID, key string) (string, error) {
	if key == "" {
		return "", nil
	}
	resolved, err := svc.ResolveDocTab(ctx, docID, key)
	if err != nil {
		return "", err
	}
	return resolved.TabID, nil
}
