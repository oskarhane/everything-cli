package service

import "context"

// SearchService is the full-text issue-search surface the `linear issue
// search` subtree consumes. It wraps Linear's searchIssues query, which
// matches on title, description, and comments and ranks by relevance.
type SearchService interface {
	SearchIssues(ctx context.Context, query string) ([]Issue, error)
}

var _ SearchService = (*Service)(nil)

// SearchIssues runs a full-text + vector search for query and returns the
// matching issues across all pages. An empty result decodes to no issues.
func (s *Service) SearchIssues(ctx context.Context, query string) ([]Issue, error) {
	const q = `query($term: String!, $first: Int, $after: String) {
		searchIssues(term: $term, first: $first, after: $after) {
			nodes { ` + issueFields + ` }
			pageInfo { hasNextPage endCursor }
		}
	}`
	variables := map[string]any{"term": query}
	return collectPages[Issue](ctx, s, q, variables, "searchIssues")
}
