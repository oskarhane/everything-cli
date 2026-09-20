package service

import (
	"context"
)

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

// CommentService is the comment surface the `linear comment` subtree consumes.
type CommentService interface {
	ListComments(ctx context.Context, issueID string) ([]Comment, error)
	CreateComment(ctx context.Context, issueID string, in CreateCommentInput) (*Comment, error)
}

// Compile-time proof that Service satisfies the comment surface.
var _ CommentService = (*Service)(nil)

// CreateCommentInput carries the fields of `linear comment create`. Body is
// required by the API; ParentID is optional — when empty the comment is
// top-level rather than a reply.
type CreateCommentInput struct {
	Body     string
	ParentID string
}

// commentFields is the selection set every comment query and mutation returns.
const commentFields = `id body createdAt updatedAt parent { id } user { id name }`

// ListComments returns every comment on the issue issueID (UUID or human
// identifier "BLA-123"), following the Relay cursor across pages.
func (s *Service) ListComments(ctx context.Context, issueID string) ([]Comment, error) {
	const query = `query($id: String!, $first: Int, $after: String) {
		issue(id: $id) {
			comments(first: $first, after: $after) {
				nodes { ` + commentFields + ` }
				pageInfo { hasNextPage endCursor }
			}
		}
	}`
	return collectPages[Comment](ctx, s, query, map[string]any{"id": issueID}, "issue", "comments")
}

// CreateComment posts a comment on issueID (UUID or human identifier
// "BLA-123"), replying to in.ParentID when set, and returns it as created.
func (s *Service) CreateComment(ctx context.Context, issueID string, in CreateCommentInput) (*Comment, error) {
	const mutation = `mutation($input: CommentCreateInput!) {
		commentCreate(input: $input) { success comment { ` + commentFields + ` } }
	}`
	input := map[string]any{"issueId": issueID, "body": in.Body}
	if in.ParentID != "" {
		input["parentId"] = in.ParentID
	}
	return mutationPayload[Comment](ctx, s, mutation, map[string]any{"input": input}, "commentCreate", "comment")
}
