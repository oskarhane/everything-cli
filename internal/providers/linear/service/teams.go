package service

import "context"

// Team is one Linear team as decoded from the GraphQL API. Key is the
// issue-identifier prefix ("BLA").
type Team struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Key  string `json:"key"`
}

// TeamService is the team surface the `linear team` subtree consumes.
type TeamService interface {
	ListTeams(ctx context.Context) ([]Team, error)
}

// ListTeams lists every team in the workspace.
func (s *Service) ListTeams(ctx context.Context) ([]Team, error) {
	const query = `query($first: Int, $after: String) {
		teams(first: $first, after: $after) {
			nodes { id name key }
			pageInfo { hasNextPage endCursor }
		}
	}`
	return collectPages[Team](ctx, s, query, nil, "teams")
}
