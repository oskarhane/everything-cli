package issue

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/google-cli/internal/providers/linear/service"
	"github.com/oskarhane/google-cli/internal/subcommands/cmdtest"
)

func TestListJSON(t *testing.T) {
	svc := &fakeService{issues: []service.Issue{seedIssue()}}
	out := cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, "json"))

	got := cmdtest.DecodeJSON(t, out)
	m, ok := got.(map[string]any)
	require.True(t, ok, "one issue renders as a single object: %v", got)
	require.Equal(t, "ENG-1", m["identifier"])
	// snake_case output fields, per the casing rule.
	require.Contains(t, m, "created_at")
	require.Contains(t, m, "updated_at")
	require.NotContains(t, m, "createdAt")
	state, ok := m["state"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "In Progress", state["name"])
	require.Equal(t, "", svc.listTeamID)
}

func TestListTable(t *testing.T) {
	svc := &fakeService{issues: []service.Issue{seedIssue()}}
	out := cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, "table"))

	// go-pretty StyleLight upper-cases header cells.
	require.Contains(t, out, "IDENTIFIER")
	require.Contains(t, out, "UPDATED_AT")
	require.Contains(t, out, "ENG-1")
	require.Contains(t, out, "In Progress")
}

func TestListPassesTeamFlag(t *testing.T) {
	svc := &fakeService{issues: []service.Issue{seedIssue()}}
	cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, "json"), "--team", "team_1")
	require.Equal(t, "team_1", svc.listTeamID)
}

func TestListToon(t *testing.T) {
	svc := &fakeService{issues: []service.Issue{seedIssue()}}
	out := cmdtest.RunCmd(t, newLeafCmd(newListCmd, svc, "toon"))
	require.Contains(t, out, "identifier: ENG-1")
}

// TestListPaginatesThroughTheLeaf runs the leaf against a mock GraphQL
// server serving two pages and asserts the combined listing renders.
func TestListPaginatesThroughTheLeaf(t *testing.T) {
	var calls []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Variables map[string]any `json:"variables"`
		}
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(body, &req))
		calls = append(calls, req.Variables)
		node := map[string]any{
			"id": "issue_1", "identifier": "ENG-1", "title": "First",
			"createdAt": "2026-08-01T10:00:00.000Z", "updatedAt": "2026-08-02T10:00:00.000Z",
		}
		page := map[string]any{"nodes": []any{node}, "pageInfo": map[string]any{"hasNextPage": false, "endCursor": ""}}
		if _, paged := req.Variables["after"]; !paged {
			node["identifier"] = "ENG-1"
			page["pageInfo"] = map[string]any{"hasNextPage": true, "endCursor": "cursor-1"}
		} else {
			node["identifier"] = "ENG-2"
		}
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"issues": page}}))
	}))
	t.Cleanup(srv.Close)

	cfg := cmdtest.NewTestConfig("json")
	out := cmdtest.RunCmd(t, newListCmd(cfg, serverSvc(srv)))

	got := cmdtest.DecodeJSON(t, out)
	arr, ok := got.([]any)
	require.True(t, ok, "two pages render as an array: %v", got)
	require.Len(t, arr, 2)
	require.Len(t, calls, 2)
	require.Equal(t, "cursor-1", calls[1]["after"])
}
