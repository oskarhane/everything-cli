package service

import "context"

// Cycle is one Linear cycle — a team's time-boxed iteration — as decoded
// from the GraphQL API. Number is the team's incrementing cycle counter.
// Name exists in the schema (verified 2026-09-22 via introspection) but is
// optional in Linear — most cycles are unnamed and decode to "".
type Cycle struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Number int    `json:"number"`
}

// CycleService is the cycle surface the `linear cycle` subtree consumes.
type CycleService interface {
	ListCycles(ctx context.Context, teamID string) ([]Cycle, error)
}

// Compile-time proof that Service satisfies the cycle surface. Each concern
// file owns its own seam assertion.
var _ CycleService = (*Service)(nil)

// ListCycles lists the cycles of one team, following the team's cycles
// connection across all pages. A null team payload errors via dig rather
// than returning an empty list.
func (s *Service) ListCycles(ctx context.Context, teamID string) ([]Cycle, error) {
	const query = `query($id: String!, $first: Int, $after: String) {
		team(id: $id) {
			cycles(first: $first, after: $after) {
				nodes { id name number }
				pageInfo { hasNextPage endCursor }
			}
		}
	}`
	return collectPages[Cycle](ctx, s, query, map[string]any{"id": teamID}, "team", "cycles")
}
