package service

import (
	"context"
	"encoding/json"
	"fmt"
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

// CreateAttachmentInput carries the fields of `linear attachment create`.
// URL and Title are required by AttachmentCreateInput; Subtitle is optional
// and omitted from the mutation when empty.
type CreateAttachmentInput struct {
	URL      string
	Title    string
	Subtitle string
}

// AttachmentService is the attachment surface the `linear attachment`
// subtree consumes.
type AttachmentService interface {
	CreateAttachment(ctx context.Context, issueID string, in CreateAttachmentInput) (*Attachment, error)
}

// Compile-time proof that Service satisfies the attachment surface.
var _ AttachmentService = (*Service)(nil)

// attachmentFields is the selection set attachmentCreate returns.
const attachmentFields = `id title url subtitle createdAt updatedAt`

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
	data, err := s.exec(ctx, mutation, map[string]any{"input": input})
	if err != nil {
		return nil, err
	}
	raw, err := dig(data, "attachmentCreate")
	if err != nil {
		return nil, err
	}
	var payload struct {
		Success    bool        `json:"success"`
		Attachment *Attachment `json:"attachment"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decoding attachmentCreate payload: %w", err)
	}
	if !payload.Success || payload.Attachment == nil {
		return nil, fmt.Errorf("linear API reported attachmentCreate success: false")
	}
	return payload.Attachment, nil
}
