package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// pageSize is the per-request page size for Linear list calls.
const pageSize = 50

// maxListPages caps how many pages one listing may follow before giving up.
// A well-behaved server ends with hasNextPage false, so the cap only ever
// fires on a misbehaving (or adversarial) endpoint looping cursors forever.
const maxListPages = 1000

// pageInfo is the Relay pagination block of a Linear connection.
type pageInfo struct {
	HasNextPage bool   `json:"hasNextPage"`
	EndCursor   string `json:"endCursor"`
}

// connection is the Relay connection shape Linear lists return.
type connection[T any] struct {
	Nodes    []T      `json:"nodes"`
	PageInfo pageInfo `json:"pageInfo"`
}

// collectPages is the one cursor-pagination loop for every Linear list: it
// pages the connection at path (e.g. "issues", or "team","issues") with
// first/after, following pageInfo.endCursor while hasNextPage is true, and
// returns every item across all pages.
func collectPages[T any](ctx context.Context, s *Service, query string, variables map[string]any, path ...string) ([]T, error) {
	var items []T
	after := ""
	for pages := 0; ; pages++ {
		vars := make(map[string]any, len(variables)+2)
		for k, v := range variables {
			vars[k] = v
		}
		vars["first"] = pageSize
		if after != "" {
			vars["after"] = after
		}
		data, err := s.exec(ctx, query, vars)
		if err != nil {
			return nil, err
		}
		raw, err := dig(data, path...)
		if err != nil {
			return nil, err
		}
		var conn connection[T]
		if err := json.Unmarshal(raw, &conn); err != nil {
			return nil, fmt.Errorf("decoding %s page: %w", strings.Join(path, "."), err)
		}
		items = append(items, conn.Nodes...)
		// hasNextPage without a cursor cannot be followed; treat the
		// listing as exhausted rather than loop on the same page.
		if !conn.PageInfo.HasNextPage || conn.PageInfo.EndCursor == "" {
			return items, nil
		}
		if pages >= maxListPages {
			return nil, fmt.Errorf("listing did not terminate after %d pages", maxListPages)
		}
		after = conn.PageInfo.EndCursor
	}
}
