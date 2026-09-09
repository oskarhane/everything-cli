package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetViewer(t *testing.T) {
	srv, calls := mockGraphQL(t, func(gqlCall) any {
		return map[string]any{"viewer": map[string]any{
			"id": "user_1", "name": "Ada Lovelace", "email": "ada@example.com",
		}}
	})
	svc := newTestService(srv)

	viewer, err := svc.GetViewer(context.Background())
	require.NoError(t, err)
	require.Equal(t, "user_1", viewer.ID)
	require.Equal(t, "Ada Lovelace", viewer.Name)
	require.Equal(t, "ada@example.com", viewer.Email)

	// The query asks for exactly the fields Viewer decodes, and carries no
	// variables — the viewer is implied by the API key on the transport.
	require.Len(t, *calls, 1)
	require.Contains(t, (*calls)[0].Query, "viewer { id name email }")
	require.Empty(t, (*calls)[0].Variables)
}

func TestGetViewerGraphQLErrors(t *testing.T) {
	srv, _ := mockGraphQL(t, func(gqlCall) any {
		return gqlErrors{{"message": "Invalid API key"}}
	})
	svc := newTestService(srv)

	_, err := svc.GetViewer(context.Background())
	require.ErrorContains(t, err, "Invalid API key")
}

func TestGetViewerNull(t *testing.T) {
	srv, _ := mockGraphQL(t, func(gqlCall) any {
		return map[string]any{"viewer": nil}
	})
	svc := newTestService(srv)

	_, err := svc.GetViewer(context.Background())
	require.ErrorContains(t, err, "linear API reported no viewer")
}
