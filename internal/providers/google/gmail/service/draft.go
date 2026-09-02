package service

import (
	"context"
	"fmt"

	gmail "google.golang.org/api/gmail/v1"
)

// DraftService is the Gmail API surface used by the draft leaves: thin
// ctx-first wrappers over Users.Drafts, so fakes model drafts, not call
// objects. CreateDraft takes the *gmail.Draft carrying the composed message
// in its base64url Raw field; SendDraft takes the draft id to send and
// returns the sent message.
//
// The real implementation is *realGmailService (service.go): the same value
// backs this interface and GmailService, and draft leaves reach it by
// asserting the GmailService the parent hands them — so the label seam stays
// untouched.
type DraftService interface {
	ListDrafts(ctx context.Context, maxResults int64) ([]*gmail.Draft, error)
	GetDraft(ctx context.Context, id string) (*gmail.Draft, error)
	CreateDraft(ctx context.Context, draft *gmail.Draft) (*gmail.Draft, error)
	SendDraft(ctx context.Context, draft *gmail.Draft) (*gmail.Message, error)
	DeleteDraft(ctx context.Context, id string) error
}

func (s *realGmailService) ListDrafts(ctx context.Context, maxResults int64) ([]*gmail.Draft, error) {
	if maxResults <= 0 {
		maxResults = 25
	}
	return collectPages(maxResults, func(page string, remaining int64) ([]*gmail.Draft, string, error) {
		call := s.svc.Users.Drafts.List(userID).Context(ctx).
			MaxResults(remaining)
		if page != "" {
			call = call.PageToken(page)
		}
		resp, err := call.Do()
		if err != nil {
			return nil, "", fmt.Errorf("listing drafts: %w", err)
		}
		return resp.Drafts, resp.NextPageToken, nil
	})
}

func (s *realGmailService) GetDraft(ctx context.Context, id string) (*gmail.Draft, error) {
	draft, err := s.svc.Users.Drafts.Get(userID, id).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("getting draft %s: %w", id, err)
	}
	return draft, nil
}

func (s *realGmailService) CreateDraft(ctx context.Context, draft *gmail.Draft) (*gmail.Draft, error) {
	created, err := s.svc.Users.Drafts.Create(userID, draft).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("creating draft: %w", err)
	}
	return created, nil
}

func (s *realGmailService) SendDraft(ctx context.Context, draft *gmail.Draft) (*gmail.Message, error) {
	sent, err := s.svc.Users.Drafts.Send(userID, draft).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("sending draft %s: %w", draft.Id, err)
	}
	return sent, nil
}

func (s *realGmailService) DeleteDraft(ctx context.Context, id string) error {
	if err := s.svc.Users.Drafts.Delete(userID, id).Context(ctx).Do(); err != nil {
		return fmt.Errorf("deleting draft %s: %w", id, err)
	}
	return nil
}
