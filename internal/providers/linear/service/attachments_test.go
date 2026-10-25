package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// attachmentNode returns a mock attachment node.
func attachmentNode(id, title, url, subtitle string) map[string]any {
	return map[string]any{
		"id": id, "title": title, "url": url, "subtitle": subtitle,
		"createdAt": "2026-08-01T12:00:00.000Z", "updatedAt": "2026-08-01T12:00:00.000Z",
	}
}

func TestCreateAttachment(t *testing.T) {
	srv, calls := mockGraphQL(t, func(gqlCall) any {
		return map[string]any{"attachmentCreate": map[string]any{
			"success":    true,
			"attachment": attachmentNode("att_1", "Design doc", "https://example.com/doc", "spec"),
		}}
	})
	svc := newTestService(srv)

	att, err := svc.CreateAttachment(context.Background(), "ENG-1", CreateAttachmentInput{
		URL:      "https://example.com/doc",
		Title:    "Design doc",
		Subtitle: "spec",
	})
	require.NoError(t, err)
	require.Equal(t, "att_1", att.ID)
	require.Equal(t, "Design doc", att.Title)
	require.Equal(t, "https://example.com/doc", att.URL)
	require.Equal(t, "spec", att.Subtitle)
	require.Equal(t, "2026-08-01T12:00:00.000Z", att.CreatedAt)
	require.Equal(t, "2026-08-01T12:00:00.000Z", att.UpdatedAt)

	require.Len(t, *calls, 1)
	require.Contains(t, (*calls)[0].Query, "attachmentCreate(input: $input)")
	require.Contains(t, (*calls)[0].Query, "id title url subtitle createdAt updatedAt")
	require.Equal(t, map[string]any{"input": map[string]any{
		"issueId":  "ENG-1",
		"url":      "https://example.com/doc",
		"title":    "Design doc",
		"subtitle": "spec",
	}}, (*calls)[0].Variables)
}

func TestCreateAttachmentOmitsEmptySubtitle(t *testing.T) {
	srv, calls := mockGraphQL(t, func(gqlCall) any {
		return map[string]any{"attachmentCreate": map[string]any{
			"success":    true,
			"attachment": attachmentNode("att_1", "Design doc", "https://example.com/doc", ""),
		}}
	})
	svc := newTestService(srv)

	_, err := svc.CreateAttachment(context.Background(), "ENG-1", CreateAttachmentInput{
		URL:   "https://example.com/doc",
		Title: "Design doc",
	})
	require.NoError(t, err)

	input, ok := (*calls)[0].Variables["input"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, map[string]any{
		"issueId": "ENG-1",
		"url":     "https://example.com/doc",
		"title":   "Design doc",
	}, input)
	require.NotContains(t, input, "subtitle")
}

func TestCreateAttachmentFailureSurfaces(t *testing.T) {
	srv, _ := mockGraphQL(t, func(gqlCall) any {
		return map[string]any{"attachmentCreate": map[string]any{"success": false, "attachment": nil}}
	})
	svc := newTestService(srv)

	_, err := svc.CreateAttachment(context.Background(), "ENG-1", CreateAttachmentInput{
		URL:   "https://example.com/doc",
		Title: "Design doc",
	})
	require.ErrorContains(t, err, "attachmentCreate")
	require.ErrorContains(t, err, "success: false")
}

// TestListAttachmentsFollowsCursorAcrossTwoPages covers the multi-page
// decode: both pages' attachments are returned, the second request follows
// pageInfo.endCursor, and the issue id rides along on every page.
func TestListAttachmentsFollowsCursorAcrossTwoPages(t *testing.T) {
	srv, calls := mockGraphQL(t, func(call gqlCall) any {
		if _, paged := call.Variables["after"]; !paged {
			return map[string]any{"issue": map[string]any{"attachments": conn([]any{
				attachmentNode("att_1", "RFC", "https://example.com/rfc", "Design doc"),
				attachmentNode("att_2", "Changelog", "https://example.com/changelog", ""),
			}, true, "cursor-1")}}
		}
		return map[string]any{"issue": map[string]any{"attachments": conn([]any{
			attachmentNode("att_3", "Runbook", "https://example.com/runbook", "on-call"),
		}, false, "")}}
	})
	svc := newTestService(srv)

	attachments, err := svc.ListAttachments(context.Background(), "ENG-1")
	require.NoError(t, err)
	require.Equal(t, []Attachment{
		{
			ID: "att_1", Title: "RFC", URL: "https://example.com/rfc", Subtitle: "Design doc",
			CreatedAt: "2026-08-01T12:00:00.000Z", UpdatedAt: "2026-08-01T12:00:00.000Z",
		},
		{
			ID: "att_2", Title: "Changelog", URL: "https://example.com/changelog",
			CreatedAt: "2026-08-01T12:00:00.000Z", UpdatedAt: "2026-08-01T12:00:00.000Z",
		},
		{
			ID: "att_3", Title: "Runbook", URL: "https://example.com/runbook", Subtitle: "on-call",
			CreatedAt: "2026-08-01T12:00:00.000Z", UpdatedAt: "2026-08-01T12:00:00.000Z",
		},
	}, attachments)

	// Two requests: the second follows pageInfo.endCursor, and both send
	// the issue id and page size.
	require.Len(t, *calls, 2)
	require.NotContains(t, (*calls)[0].Variables, "after")
	require.Equal(t, "cursor-1", (*calls)[1].Variables["after"])
	for _, c := range *calls {
		require.Equal(t, "ENG-1", c.Variables["id"])
		require.Equal(t, float64(pageSize), c.Variables["first"])
	}
	require.Contains(t, (*calls)[0].Query, "issue(id: $id)")
	require.Contains(t, (*calls)[0].Query, "attachments(first: $first, after: $after)")
	require.Contains(t, (*calls)[0].Query, "nodes { id title url subtitle createdAt updatedAt }")
}

// TestListAttachmentsEmpty asserts an empty connection decodes to no rows
// in a single request.
func TestListAttachmentsEmpty(t *testing.T) {
	srv, calls := mockGraphQL(t, func(gqlCall) any {
		return map[string]any{"issue": map[string]any{"attachments": conn([]any{}, false, "")}}
	})
	svc := newTestService(srv)

	attachments, err := svc.ListAttachments(context.Background(), "issue_2")
	require.NoError(t, err)
	require.Empty(t, attachments)
	require.Len(t, *calls, 1)
}
