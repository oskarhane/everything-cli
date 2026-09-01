package service

import (
	"context"
	"fmt"

	gmail "google.golang.org/api/gmail/v1"
)

// ThreadService is the Gmail API surface used by the thread leaves: thin
// ctx-first wrappers over Users.Threads, so fakes model threads, not call
// objects. ListThreads takes the composed Gmail search query (the q
// parameter), the label IDs every result must carry, and pages until
// maxResults threads are collected. GetThread returns the thread with its
// messages in the API's "full" format.
//
// The real implementation is *realGmailService (service.go): the same value
// backs this interface and GmailService, and thread leaves reach it by
// asserting the GmailService the parent hands them — so the label seam stays
// untouched.
type ThreadService interface {
	ListThreads(ctx context.Context, q string, labelIDs []string, maxResults int64) ([]*gmail.Thread, error)
	GetThread(ctx context.Context, id string) (*gmail.Thread, error)
}

func (s *realGmailService) ListThreads(ctx context.Context, q string, labelIDs []string, maxResults int64) ([]*gmail.Thread, error) {
	if maxResults <= 0 {
		maxResults = 25
	}
	return collectPages(maxResults, func(page string, remaining int64) ([]*gmail.Thread, string, error) {
		call := s.svc.Users.Threads.List(userID).Context(ctx).
			MaxResults(remaining).
			LabelIds(labelIDs...)
		if q != "" {
			call = call.Q(q)
		}
		if page != "" {
			call = call.PageToken(page)
		}
		resp, err := call.Do()
		if err != nil {
			return nil, "", fmt.Errorf("listing threads: %w", err)
		}
		return resp.Threads, resp.NextPageToken, nil
	})
}

func (s *realGmailService) GetThread(ctx context.Context, id string) (*gmail.Thread, error) {
	thread, err := s.svc.Users.Threads.Get(userID, id).Context(ctx).Format("full").Do()
	if err != nil {
		return nil, fmt.Errorf("getting thread %s: %w", id, err)
	}
	return thread, nil
}
