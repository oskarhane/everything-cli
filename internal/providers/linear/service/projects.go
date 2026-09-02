package service

import "context"

// Project is one Linear project as decoded from the GraphQL API.
type Project struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	State       string `json:"state"`
}

// ProjectService is the project surface the `linear project` subtree
// consumes.
type ProjectService interface {
	ListProjects(ctx context.Context) ([]Project, error)
}

// ListProjects lists every project in the workspace.
func (s *Service) ListProjects(ctx context.Context) ([]Project, error) {
	const query = `query($first: Int, $after: String) {
		projects(first: $first, after: $after) {
			nodes { id name description state }
			pageInfo { hasNextPage endCursor }
		}
	}`
	return collectPages[Project](ctx, s, query, nil, "projects")
}
