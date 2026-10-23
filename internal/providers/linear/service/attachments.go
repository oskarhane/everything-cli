package service

import (
	"context"
)

// Attachment is one issue link attachment as decoded from the GraphQL API.
// The JSON tags are the wire shape (camelCase); a wire null decodes to the
// empty string.
type Attachment struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	URL       string `json:"url"`
	Subtitle  string `json:"subtitle"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// CreateAttachmentInput carries the fields of `linear issue attachment create`.
// URL and Title are required by AttachmentCreateInput; Subtitle is optional
// and omitted from the mutation when empty.
type CreateAttachmentInput struct {
	URL      string
	Title    string
	Subtitle string
}

// AttachmentService is the attachment surface the `linear issue attachment`
// subtree consumes.
type AttachmentService interface {
	ListAttachments(ctx context.Context, issueID string) ([]Attachment, error)
	CreateAttachment(ctx context.Context, issueID string, in CreateAttachmentInput) (*Attachment, error)
}

// Compile-time proof that Service satisfies the attachment surface.
var _ AttachmentService = (*Service)(nil)

// attachmentFields is the selection set every attachment query and mutation
// returns.
const attachmentFields = `id title url subtitle createdAt updatedAt`

// ListAttachments returns every link attachment on the issue issueID (UUID
// or human identifier "BLA-123"), following the Relay cursor across pages.
func (s *Service) ListAttachments(ctx context.Context, issueID string) ([]Attachment, error) {
	const query = `query($id: String!, $first: Int, $after: String) {
		issue(id: $id) {
			attachments(first: $first, after: $after) {
				nodes { ` + attachmentFields + ` }
				pageInfo { hasNextPage endCursor }
			}
		}
	}`
	return collectPages[Attachment](ctx, s, query, map[string]any{"id": issueID}, "issue", "attachments")
}

// CreateAttachment links a URL to the issue issueID (UUID or human
// identifier "BLA-123"). Linear treats a repeated url + issueId as an update
// to the existing attachment, so re-posting is idempotent by design.
func (s *Service) CreateAttachment(ctx context.Context, issueID string, in CreateAttachmentInput) (*Attachment, error) {
	const mutation = `mutation($input: AttachmentCreateInput!) {
		attachmentCreate(input: $input) { success attachment { ` + attachmentFields + ` } }
	}`
	input := map[string]any{"issueId": issueID, "url": in.URL, "title": in.Title}
	if in.Subtitle != "" {
		input["subtitle"] = in.Subtitle
	}
	return mutationPayload[Attachment](ctx, s, mutation, map[string]any{"input": input}, "attachmentCreate", "attachment")
}
