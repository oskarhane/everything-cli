package service

import "context"

// MilestoneService is the project-milestone surface the `linear milestone`
// subtree consumes.
type MilestoneService interface {
	ListProjectMilestones(ctx context.Context, projectID string) ([]NamedRef, error)
}

// Compile-time proof that Service satisfies the milestone surface. Each
// concern file owns its own seam assertion.
var _ MilestoneService = (*Service)(nil)

// ListProjectMilestones lists the milestones of one project, following the
// project's projectMilestones connection across all pages. A null project
// payload errors via dig rather than returning an empty list. Milestones
// decode into NamedRef: the query selects exactly id and name.
func (s *Service) ListProjectMilestones(ctx context.Context, projectID string) ([]NamedRef, error) {
	const query = `query($id: String!, $first: Int, $after: String) {
		project(id: $id) {
			projectMilestones(first: $first, after: $after) {
				nodes { id name }
				pageInfo { hasNextPage endCursor }
			}
		}
	}`
	return collectPages[NamedRef](ctx, s, query, map[string]any{"id": projectID}, "project", "projectMilestones")
}
