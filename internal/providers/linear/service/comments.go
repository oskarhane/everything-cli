package service

import "context"

// IDRef is a linked Linear object reference identified by ID only, such as
// a comment's parent comment.
type IDRef struct {
	ID string `json:"id"`
}

// Comment is one Linear issue comment as decoded from the GraphQL API. The
// JSON tags are the wire shape (camelCase); a wire null decodes to the zero
// value (nil refs).
type Comment struct {
	ID        string    `json:"id"`
	Body      string    `json:"body"`
	CreatedAt string    `json:"createdAt"`
	UpdatedAt string    `json:"updatedAt"`
	Parent    *IDRef    `json:"parent"`
	User      *NamedRef `json:"user"`
}

// ListComments returns every comment on the issue issueID (UUID or human
// identifier "BLA-123"), following the Relay cursor across pages.
func (s *Service) ListComments(ctx context.Context, issueID string) ([]Comment, error) {
	const query = `query($id: String!, $first: Int, $after: String) {
		issue(id: $id) {
			comments(first: $first, after: $after) {
				nodes { id body createdAt updatedAt parent { id } user { id name } }
				pageInfo { hasNextPage endCursor }
			}
		}
	}`
	return collectPages[Comment](ctx, s, query, map[string]any{"id": issueID}, "issue", "comments")
}
