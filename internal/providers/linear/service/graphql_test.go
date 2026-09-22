package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExecGraphQLReturnsRawDataDocument(t *testing.T) {
	srv, calls := mockGraphQL(t, func(gqlCall) any {
		return map[string]any{"viewer": map[string]any{"id": "user_1", "name": "Ada"}}
	})
	svc := newTestService(srv)

	data, err := svc.ExecGraphQL(context.Background(),
		"{ viewer { id name } }", map[string]any{"limit": 1})
	require.NoError(t, err)
	require.JSONEq(t, `{"viewer":{"id":"user_1","name":"Ada"}}`, string(data))

	// The query and variables pass through to the wire untouched, with the
	// API key on the transport.
	require.Len(t, *calls, 1)
	require.Equal(t, "{ viewer { id name } }", (*calls)[0].Query)
	require.Equal(t, map[string]any{"limit": float64(1)}, (*calls)[0].Variables)
	require.Equal(t, "test-key-123", (*calls)[0].Auth)
}

func TestExecGraphQLErrorsArraySurfaces(t *testing.T) {
	srv, _ := mockGraphQL(t, func(gqlCall) any {
		return gqlErrors{
			{"message": "Field \"nope\" doesn't exist", "extensions": map[string]any{"code": "GRAPHQL_VALIDATION_FAILED"}},
			{"message": "second problem"},
		}
	})
	svc := newTestService(srv)

	_, err := svc.ExecGraphQL(context.Background(), "{ nope }", nil)
	require.ErrorContains(t, err, `Field "nope" doesn't exist`)
	require.ErrorContains(t, err, "GRAPHQL_VALIDATION_FAILED")
	require.ErrorContains(t, err, "second problem")
}

func TestExecGraphQLNoVariables(t *testing.T) {
	srv, calls := mockGraphQL(t, func(gqlCall) any {
		return map[string]any{"teams": conn([]any{}, false, "")}
	})
	svc := newTestService(srv)

	data, err := svc.ExecGraphQL(context.Background(), "{ teams { nodes { id } } }", nil)
	require.NoError(t, err)
	require.JSONEq(t, `{"teams":{"nodes":[],"pageInfo":{"hasNextPage":false,"endCursor":""}}}`, string(data))
	require.Len(t, *calls, 1)
	require.Empty(t, (*calls)[0].Variables)
}
