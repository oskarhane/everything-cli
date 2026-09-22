package service

import "context"

// Label is one Linear issue label as decoded from the GraphQL API. Color is
// the hex color Linear renders the label chip with ("#ff0000").
type Label struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

// LabelService is the label surface the `linear label` subtree consumes.
type LabelService interface {
	ListTeamLabels(ctx context.Context, teamID string) ([]Label, error)
}

// Compile-time proof that Service satisfies the label surface. Each concern
// file owns its own seam assertion.
var _ LabelService = (*Service)(nil)

// ListTeamLabels lists the labels of one team, paginating the team.labels
// connection. A null team leaves no labels to walk into; dig names the null
// intermediate segment.
func (s *Service) ListTeamLabels(ctx context.Context, teamID string) ([]Label, error) {
	const query = `query($id: String!, $first: Int, $after: String) {
		team(id: $id) {
			labels(first: $first, after: $after) {
				nodes { id name color }
				pageInfo { hasNextPage endCursor }
			}
		}
	}`
	return collectPages[Label](ctx, s, query, map[string]any{"id": teamID}, "team", "labels")
}
