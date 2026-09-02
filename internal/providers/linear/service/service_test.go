package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/output"
)

// TestMain neutralizes format auto-detection so the host's harness env and
// TTY cannot flip output expectations.
func TestMain(m *testing.M) {
	output.IsAgent = func() bool { return false }
	output.StdoutIsTerminal = func() bool { return false }
	os.Exit(m.Run())
}

// gqlCall records one request the mock GraphQL server received.
type gqlCall struct {
	Query     string
	Variables map[string]any
	Auth      string
}

// mockGraphQL starts a hermetic GraphQL server. respond maps each recorded
// call to the data document to return; returning the error marker fails the
// request with a GraphQL errors array instead.
func mockGraphQL(t *testing.T, respond func(call gqlCall) any) (*httptest.Server, *[]gqlCall) {
	t.Helper()
	calls := &[]gqlCall{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var req struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		require.NoError(t, json.Unmarshal(body, &req))
		call := gqlCall{Query: req.Query, Variables: req.Variables, Auth: r.Header.Get("Authorization")}
		*calls = append(*calls, call)
		w.Header().Set("Content-Type", "application/json")
		switch data := respond(call).(type) {
		case gqlErrors:
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"errors": data}))
		default:
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": data}))
		}
	}))
	t.Cleanup(srv.Close)
	return srv, calls
}

// gqlErrors marks a mock response as a GraphQL errors array.
type gqlErrors []map[string]any

// authTransport stamps an API key on every request, mirroring the apikey
// strategy's transport, so tests can assert the key reaches the server.
type authTransport struct{ key string }

func (t authTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	clone := r.Clone(r.Context())
	clone.Header.Set("Authorization", t.key)
	return http.DefaultTransport.RoundTrip(clone)
}

// newTestService returns the real service pointed at the mock server, with
// a fake API key on the transport.
func newTestService(srv *httptest.Server) *Service {
	return NewForEndpoint(&http.Client{Transport: authTransport{key: "test-key-123"}}, srv.URL)
}

// issueNode returns a mock issue node.
func issueNode(id, identifier, title string) map[string]any {
	return map[string]any{
		"id": id, "identifier": identifier, "title": title,
		"description": "", "url": "https://linear.app/x/issue/" + identifier,
		"createdAt": "2026-08-01T10:00:00.000Z", "updatedAt": "2026-08-02T10:00:00.000Z",
		"state":    map[string]any{"id": "state_1", "name": "In Progress"},
		"assignee": map[string]any{"id": "user_1", "name": "Ada"},
		"team":     map[string]any{"id": "team_1", "name": "Engineering", "key": "ENG"},
	}
}

// conn wraps nodes in a Relay connection document.
func conn(nodes []any, hasNext bool, endCursor string) map[string]any {
	return map[string]any{
		"nodes":    nodes,
		"pageInfo": map[string]any{"hasNextPage": hasNext, "endCursor": endCursor},
	}
}

func TestListIssuesFollowsCursorAcrossTwoPages(t *testing.T) {
	srv, calls := mockGraphQL(t, func(call gqlCall) any {
		if _, paged := call.Variables["after"]; !paged {
			return map[string]any{"issues": conn([]any{
				issueNode("issue_1", "ENG-1", "First"),
				issueNode("issue_2", "ENG-2", "Second"),
			}, true, "cursor-1")}
		}
		return map[string]any{"issues": conn([]any{
			issueNode("issue_3", "ENG-3", "Third"),
		}, false, "")}
	})
	svc := newTestService(srv)

	issues, err := svc.ListIssues(context.Background(), "")
	require.NoError(t, err)
	require.Len(t, issues, 3)
	require.Equal(t, "ENG-1", issues[0].Identifier)
	require.Equal(t, "ENG-3", issues[2].Identifier)
	require.Equal(t, "Ada", issues[0].Assignee.Name)
	require.Equal(t, "ENG", issues[0].Team.Key)

	// Two requests: the second follows pageInfo.endCursor, and both carry
	// the raw API key (no Bearer prefix).
	require.Len(t, *calls, 2)
	require.NotContains(t, (*calls)[0].Variables, "after")
	require.Equal(t, "cursor-1", (*calls)[1].Variables["after"])
	require.Equal(t, float64(pageSize), (*calls)[0].Variables["first"])
	for _, c := range *calls {
		require.Equal(t, "test-key-123", c.Auth)
	}
}

func TestListIssuesScopedToTeam(t *testing.T) {
	srv, calls := mockGraphQL(t, func(gqlCall) any {
		return map[string]any{"team": map[string]any{
			"issues": conn([]any{issueNode("issue_1", "ENG-1", "First")}, false, ""),
		}}
	})
	svc := newTestService(srv)

	issues, err := svc.ListIssues(context.Background(), "team_1")
	require.NoError(t, err)
	require.Len(t, issues, 1)
	require.Len(t, *calls, 1)
	require.Equal(t, "team_1", (*calls)[0].Variables["teamId"])
	require.Contains(t, (*calls)[0].Query, "team(id: $teamId)")
}

func TestGetIssue(t *testing.T) {
	srv, calls := mockGraphQL(t, func(gqlCall) any {
		return map[string]any{"issue": issueNode("issue_1", "ENG-1", "First")}
	})
	svc := newTestService(srv)

	issue, err := svc.GetIssue(context.Background(), "ENG-1")
	require.NoError(t, err)
	require.Equal(t, "ENG-1", issue.Identifier)
	require.Equal(t, "In Progress", issue.State.Name)
	require.Equal(t, "ENG-1", (*calls)[0].Variables["id"])
}

func TestGetIssueNotFound(t *testing.T) {
	srv, _ := mockGraphQL(t, func(gqlCall) any {
		return map[string]any{"issue": nil}
	})
	svc := newTestService(srv)

	_, err := svc.GetIssue(context.Background(), "ENG-999")
	require.ErrorContains(t, err, `issue "ENG-999" not found`)
}

func TestCreateIssue(t *testing.T) {
	srv, calls := mockGraphQL(t, func(gqlCall) any {
		return map[string]any{"issueCreate": map[string]any{
			"success": true, "issue": issueNode("issue_new", "ENG-4", "Fourth"),
		}}
	})
	svc := newTestService(srv)

	issue, err := svc.CreateIssue(context.Background(), CreateIssueInput{
		TeamID:      "team_1",
		Title:       "Fourth",
		Description: "details",
		StateID:     "state_1",
	})
	require.NoError(t, err)
	require.Equal(t, "ENG-4", issue.Identifier)

	input, ok := (*calls)[0].Variables["input"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "team_1", input["teamId"])
	require.Equal(t, "Fourth", input["title"])
	require.Equal(t, "details", input["description"])
	require.Equal(t, "state_1", input["stateId"])
	// Optional fields left empty are omitted from the mutation.
	require.NotContains(t, input, "assigneeId")
}

func TestUpdateIssueSendsOnlyChangedFields(t *testing.T) {
	srv, calls := mockGraphQL(t, func(gqlCall) any {
		return map[string]any{"issueUpdate": map[string]any{
			"success": true, "issue": issueNode("issue_1", "ENG-1", "Retitled"),
		}}
	})
	svc := newTestService(srv)

	issue, err := svc.UpdateIssue(context.Background(), "ENG-1", UpdateIssueInput{Title: "Retitled"})
	require.NoError(t, err)
	require.Equal(t, "Retitled", issue.Title)

	require.Equal(t, "ENG-1", (*calls)[0].Variables["id"])
	input, ok := (*calls)[0].Variables["input"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, map[string]any{"title": "Retitled"}, input)
}

func TestListTeamsPaginates(t *testing.T) {
	srv, calls := mockGraphQL(t, func(call gqlCall) any {
		if _, paged := call.Variables["after"]; !paged {
			return map[string]any{"teams": conn([]any{
				map[string]any{"id": "team_1", "name": "Engineering", "key": "ENG"},
			}, true, "cursor-1")}
		}
		return map[string]any{"teams": conn([]any{
			map[string]any{"id": "team_2", "name": "Design", "key": "DES"},
		}, false, "")}
	})
	svc := newTestService(srv)

	teams, err := svc.ListTeams(context.Background())
	require.NoError(t, err)
	require.Len(t, teams, 2)
	require.Equal(t, "DES", teams[1].Key)
	require.Len(t, *calls, 2)
	require.Equal(t, "cursor-1", (*calls)[1].Variables["after"])
}

func TestListProjects(t *testing.T) {
	srv, _ := mockGraphQL(t, func(gqlCall) any {
		return map[string]any{"projects": conn([]any{
			map[string]any{"id": "proj_1", "name": "Rewrite", "description": "", "state": "started"},
		}, false, "")}
	})
	svc := newTestService(srv)

	projects, err := svc.ListProjects(context.Background())
	require.NoError(t, err)
	require.Len(t, projects, 1)
	require.Equal(t, "started", projects[0].State)
}

func TestGraphQLErrorsSurface(t *testing.T) {
	srv, _ := mockGraphQL(t, func(gqlCall) any {
		return gqlErrors{{
			"message":    "Complexity limit exceeded",
			"extensions": map[string]any{"code": "RATELIMITED"},
		}}
	})
	svc := newTestService(srv)

	_, err := svc.ListTeams(context.Background())
	require.ErrorContains(t, err, "Complexity limit exceeded")
	require.ErrorContains(t, err, "RATELIMITED")
}

func TestNon200Surfaces(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "bad gateway", http.StatusBadGateway)
	}))
	t.Cleanup(srv.Close)
	svc := newTestService(srv)

	_, err := svc.ListTeams(context.Background())
	require.ErrorContains(t, err, "502")
}

func TestNon200ErrorBodyIsTruncated(t *testing.T) {
	// A hostile endpoint echoing an unbounded error body must not flood the
	// error string: the read is capped and the cut marked with an ellipsis.
	big := strings.Repeat("x", maxErrBodyBytes*4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(big))
	}))
	t.Cleanup(srv.Close)
	svc := newTestService(srv)

	_, err := svc.ListTeams(context.Background())
	require.Error(t, err)
	require.ErrorContains(t, err, "500")
	require.NotContains(t, err.Error(), big, "the full body must not be echoed")
	require.Contains(t, err.Error(), strings.Repeat("x", maxErrBodyBytes)+"...")
}

func TestRunawayPaginationIsCapped(t *testing.T) {
	srv, calls := mockGraphQL(t, func(gqlCall) any {
		// A misbehaving endpoint that never ends: hasNextPage forever.
		return map[string]any{"teams": conn([]any{
			map[string]any{"id": "team_1", "name": "Engineering", "key": "ENG"},
		}, true, "cursor-loop")}
	})
	svc := newTestService(srv)

	_, err := svc.ListTeams(context.Background())
	require.ErrorContains(t, err, "did not terminate")
	require.LessOrEqual(t, len(*calls), maxListPages+1)
}

func TestMutationFailureSurfaces(t *testing.T) {
	srv, _ := mockGraphQL(t, func(gqlCall) any {
		return map[string]any{"issueCreate": map[string]any{"success": false, "issue": nil}}
	})
	svc := newTestService(srv)

	_, err := svc.CreateIssue(context.Background(), CreateIssueInput{TeamID: "team_1", Title: "X"})
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "success: false"))
}
