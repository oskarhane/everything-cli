package service

import (
	"context"
	"fmt"

	gmail "google.golang.org/api/gmail/v1"
)

// MessageService is the Gmail API surface used by the message leaves: thin
// ctx-first wrappers over Users.Messages, so fakes model messages, not call
// objects. ListMessages takes a composed Gmail search query (the q parameter)
// and pages until maxResults messages are collected or the pages run out.
// GetMessage takes the API format ("full" for parsed payload headers, "raw"
// for the base64url RFC 2822 message).
//
// The real implementation is *realGmailService (service.go): the same value
// backs both this interface and GmailService, and message leaves reach it by
// asserting the GmailService the parent hands them — so the label seam stays
// untouched.
type MessageService interface {
	ListMessages(ctx context.Context, q string, maxResults int64) ([]*gmail.Message, error)
	GetMessage(ctx context.Context, id, format string) (*gmail.Message, error)
	SendMessage(ctx context.Context, message *gmail.Message) (*gmail.Message, error)
	TrashMessage(ctx context.Context, id string) (*gmail.Message, error)
	UntrashMessage(ctx context.Context, id string) (*gmail.Message, error)
	DeleteMessage(ctx context.Context, id string) error
	ModifyMessage(ctx context.Context, id string, req *gmail.ModifyMessageRequest) (*gmail.Message, error)
}

// maxPageSize is the Gmail API's per-request cap on maxResults.
const maxPageSize = 500

func (s *realGmailService) ListMessages(ctx context.Context, q string, maxResults int64) ([]*gmail.Message, error) {
	if maxResults <= 0 {
		maxResults = 100
	}
	var messages []*gmail.Message
	for page := ""; ; {
		call := s.svc.Users.Messages.List(userID).Context(ctx).
			MaxResults(min(maxResults-int64(len(messages)), maxPageSize))
		if q != "" {
			call = call.Q(q)
		}
		if page != "" {
			call = call.PageToken(page)
		}
		resp, err := call.Do()
		if err != nil {
			return nil, fmt.Errorf("listing messages: %w", err)
		}
		messages = append(messages, resp.Messages...)
		if resp.NextPageToken == "" || int64(len(messages)) >= maxResults {
			break
		}
		page = resp.NextPageToken
	}
	if int64(len(messages)) > maxResults {
		messages = messages[:maxResults]
	}
	return messages, nil
}

func (s *realGmailService) GetMessage(ctx context.Context, id, format string) (*gmail.Message, error) {
	call := s.svc.Users.Messages.Get(userID, id).Context(ctx)
	if format != "" {
		call = call.Format(format)
	}
	message, err := call.Do()
	if err != nil {
		return nil, fmt.Errorf("getting message %s: %w", id, err)
	}
	return message, nil
}

func (s *realGmailService) SendMessage(ctx context.Context, message *gmail.Message) (*gmail.Message, error) {
	sent, err := s.svc.Users.Messages.Send(userID, message).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("sending message: %w", err)
	}
	return sent, nil
}

func (s *realGmailService) TrashMessage(ctx context.Context, id string) (*gmail.Message, error) {
	message, err := s.svc.Users.Messages.Trash(userID, id).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("trashing message %s: %w", id, err)
	}
	return message, nil
}

func (s *realGmailService) UntrashMessage(ctx context.Context, id string) (*gmail.Message, error) {
	message, err := s.svc.Users.Messages.Untrash(userID, id).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("untrashing message %s: %w", id, err)
	}
	return message, nil
}

func (s *realGmailService) DeleteMessage(ctx context.Context, id string) error {
	if err := s.svc.Users.Messages.Delete(userID, id).Context(ctx).Do(); err != nil {
		return fmt.Errorf("deleting message %s: %w", id, err)
	}
	return nil
}

func (s *realGmailService) ModifyMessage(ctx context.Context, id string, req *gmail.ModifyMessageRequest) (*gmail.Message, error) {
	message, err := s.svc.Users.Messages.Modify(userID, id, req).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("modifying message %s: %w", id, err)
	}
	return message, nil
}
