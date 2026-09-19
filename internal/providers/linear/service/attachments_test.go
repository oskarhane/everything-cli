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
