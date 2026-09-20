package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// commentCreatedPayload wraps a mock comment in the commentCreate mutation
// payload shape.
func commentCreatedPayload(c map[string]any) map[string]any {
	return map[string]any{"commentCreate": map[string]any{"success": true, "comment": c}}
}

func TestCreateCommentSendsInputAndDecodesComment(t *testing.T) {
	srv, calls := mockGraphQL(t, func(gqlCall) any {
		return commentCreatedPayload(commentNode("comment_new", "hello"))
	})
	svc := newTestService(srv)

	comment, err := svc.CreateComment(context.Background(), "issue_1", CreateCommentInput{
		Body:     "hello",
		ParentID: "comment_0",
	})
	require.NoError(t, err)
	// The mutation's selection set decodes into the same Comment wire shape
	// ListComments returns, including the nested refs.
	require.Equal(t, "comment_new", comment.ID)
	require.Equal(t, "hello", comment.Body)
	require.Equal(t, "2026-08-01T12:00:00.000Z", comment.CreatedAt)
	require.Equal(t, "Ada", comment.User.Name)
	require.Equal(t, "comment_0", comment.Parent.ID)

	require.Len(t, *calls, 1)
	require.Contains(t, (*calls)[0].Query, "commentCreate(input: $input)")
	require.Contains(t, (*calls)[0].Query, "success comment {")
	require.Equal(t, map[string]any{
		"input": map[string]any{
			"issueId":  "issue_1",
			"body":     "hello",
			"parentId": "comment_0",
		},
	}, (*calls)[0].Variables)
}

func TestCreateCommentOmitsParentIDWhenEmpty(t *testing.T) {
	srv, calls := mockGraphQL(t, func(gqlCall) any {
		return commentCreatedPayload(commentNode("comment_new", "top level"))
	})
	svc := newTestService(srv)

	_, err := svc.CreateComment(context.Background(), "issue_1", CreateCommentInput{Body: "top level"})
	require.NoError(t, err)

	input, ok := (*calls)[0].Variables["input"].(map[string]any)
	require.True(t, ok)
	// A top-level comment must omit parentId entirely rather than send an
	// empty string, which the API would reject.
	require.NotContains(t, input, "parentId")
	require.Equal(t, map[string]any{"issueId": "issue_1", "body": "top level"}, input)
}

func TestCreateCommentSuccessFalseNamesCommentCreate(t *testing.T) {
	srv, _ := mockGraphQL(t, func(gqlCall) any {
		return map[string]any{"commentCreate": map[string]any{"success": false, "comment": nil}}
	})
	svc := newTestService(srv)

	_, err := svc.CreateComment(context.Background(), "issue_1", CreateCommentInput{Body: "hello"})
	require.ErrorContains(t, err, "commentCreate")
	require.ErrorContains(t, err, "success: false")
}
