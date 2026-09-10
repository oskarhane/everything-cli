package tabs

import (
	"context"
	"fmt"
	"strings"

	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
)

// resolveTab resolves the tab a write targets: the key matches an exact tab
// ID first, then exact titles — an ID always wins over a same-named title, a
// single title match resolves, several are ambiguous, none is unknown. The
// resolution reads the document's tab tree, so the leaf always writes the
// immutable tab ID, never the caller's key.
func resolveTab(ctx context.Context, svc service.DocService, docID, key string) (service.DocTab, error) {
	docTabs, err := svc.ListDocTabs(ctx, docID)
	if err != nil {
		return service.DocTab{}, err
	}
	for _, t := range docTabs {
		if t.TabID == key {
			return t, nil
		}
	}
	var matches []service.DocTab
	for _, t := range docTabs {
		if t.Title == key {
			matches = append(matches, t)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return service.DocTab{}, fmt.Errorf("no tab with ID or title %q", key)
	default:
		ids := make([]string, 0, len(matches))
		for _, t := range matches {
			ids = append(ids, t.TabID)
		}
		return service.DocTab{}, fmt.Errorf("tab title %q is ambiguous: matches tabs %s; use a tab ID", key, strings.Join(ids, ", "))
	}
}
