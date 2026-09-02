package service

import (
	"context"
	"fmt"

	gmail "google.golang.org/api/gmail/v1"
)

// AttachmentService is the Gmail API surface used by the attachment leaves:
// a thin ctx-first wrapper over Users.Messages.Attachments. The returned part
// body carries the attachment's data as base64url; the leaf decodes it. The
// attachment id only names a part of one message, so every call needs the
// owning message's id too.
//
// The real implementation is *realGmailService (service.go): the same value
// backs this interface and GmailService, and attachment leaves reach it by
// asserting the GmailService the parent hands them — so the label seam stays
// untouched.
type AttachmentService interface {
	GetAttachment(ctx context.Context, messageID, attachmentID string) (*gmail.MessagePartBody, error)
}

func (s *realGmailService) GetAttachment(ctx context.Context, messageID, attachmentID string) (*gmail.MessagePartBody, error) {
	part, err := s.svc.Users.Messages.Attachments.Get(userID, messageID, attachmentID).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("getting attachment %s of message %s: %w", attachmentID, messageID, err)
	}
	return part, nil
}
