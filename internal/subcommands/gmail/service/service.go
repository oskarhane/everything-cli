// Package service is the seam between the gmail command leaves and the Gmail
// API. Leaves depend on the GmailService interface so tests can run hermeti-
// cally against a fake; only this package talks to the real API.
//
// The interface covers only what the label subtree needs. Later subtrees
// (message, draft, thread, attachment) add their own interfaces in their own
// files here rather than growing this one.
package service

import (
	"context"
	"fmt"

	"golang.org/x/oauth2"
	gmail "google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

// userID is the Gmail user all commands act as: the authenticated account.
const userID = "me"

// GmailService is the Gmail API surface used by the label leaves. Thin
// wrappers over Users.Labels, so fakes only model labels, not call objects.
type GmailService interface {
	ListLabels(ctx context.Context) ([]*gmail.Label, error)
	GetLabel(ctx context.Context, id string) (*gmail.Label, error)
	CreateLabel(ctx context.Context, label *gmail.Label) (*gmail.Label, error)
	UpdateLabel(ctx context.Context, id string, label *gmail.Label) (*gmail.Label, error)
	DeleteLabel(ctx context.Context, id string) error
}

// New returns a GmailService bound to ts.
func New(ctx context.Context, ts oauth2.TokenSource) (GmailService, error) {
	svc, err := gmail.NewService(ctx, option.WithTokenSource(ts))
	if err != nil {
		return nil, fmt.Errorf("creating gmail service: %w", err)
	}
	return &realGmailService{svc: svc}, nil
}

// realGmailService adapts *gmail.Service to GmailService.
type realGmailService struct {
	svc *gmail.Service
}

func (s *realGmailService) ListLabels(ctx context.Context) ([]*gmail.Label, error) {
	resp, err := s.svc.Users.Labels.List(userID).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("listing labels: %w", err)
	}
	return resp.Labels, nil
}

func (s *realGmailService) GetLabel(ctx context.Context, id string) (*gmail.Label, error) {
	label, err := s.svc.Users.Labels.Get(userID, id).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("getting label %s: %w", id, err)
	}
	return label, nil
}

func (s *realGmailService) CreateLabel(ctx context.Context, label *gmail.Label) (*gmail.Label, error) {
	created, err := s.svc.Users.Labels.Create(userID, label).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("creating label: %w", err)
	}
	return created, nil
}

func (s *realGmailService) UpdateLabel(ctx context.Context, id string, label *gmail.Label) (*gmail.Label, error) {
	updated, err := s.svc.Users.Labels.Update(userID, id, label).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("updating label %s: %w", id, err)
	}
	return updated, nil
}

func (s *realGmailService) DeleteLabel(ctx context.Context, id string) error {
	if err := s.svc.Users.Labels.Delete(userID, id).Context(ctx).Do(); err != nil {
		return fmt.Errorf("deleting label %s: %w", id, err)
	}
	return nil
}
