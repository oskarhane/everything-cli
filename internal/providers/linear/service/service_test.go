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
		"startedAt":   "2026-08-01T11:00:00.000Z",
		"completedAt": nil, "canceledAt": nil,
		"state":    map[string]any{"id": "state_1", "name": "In Progress", "type": "started"},
		"assignee": map[string]any{"id": "user_1", "name": "Ada"},
		"creator":  map[string]any{"id": "user_2", "name": "Grace"},
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

	issues, err := svc.ListIssues(context.Background(), IssueFilter{})
	require.NoError(t, err)
	require.Len(t, issues, 3)
	require.Equal(t, "ENG-1", issues[0].Identifier)
	require.Equal(t, "ENG-3", issues[2].Identifier)
	require.Equal(t, "Ada", issues[0].Assignee.Name)
	require.Equal(t, "ENG", issues[0].Team.Key)
	// New selection fields decode: state type, creator, timestamps. A wire
	// null timestamp decodes to the empty string.
	require.Equal(t, "started", issues[0].State.Type)
	require.Equal(t, "Grace", issues[0].Creator.Name)
	require.Equal(t, "2026-08-01T11:00:00.000Z", issues[0].StartedAt)
	require.Equal(t, "", issues[0].CompletedAt)
	require.Equal(t, "", issues[0].CanceledAt)

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

// TestListIssuesFilterVariable asserts the emitted GraphQL filter variable
// for each composition of IssueFilter: id-like keys match by equality,
// UpdatedSince matches updatedAt at-or-after.
func TestListIssuesFilterVariable(t *testing.T) {
	eq := func(id string) map[string]any {
		return map[string]any{"id": map[string]any{"eq": id}}
	}
	tests := []struct {
		name   string
		filter IssueFilter
		want   map[string]any
	}{
		{
			name:   "team only",
			filter: IssueFilter{TeamID: "team_1"},
			want:   map[string]any{"team": eq("team_1")},
		},
		{
			name:   "assignee and updated since",
			filter: IssueFilter{AssigneeID: "user_1", UpdatedSince: "2026-09-01T00:00:00Z"},
			want: map[string]any{
				"assignee":  eq("user_1"),
				"updatedAt": map[string]any{"gte": "2026-09-01T00:00:00Z"},
			},
		},
		{
			name: "all four fields compose into one filter",
			filter: IssueFilter{
				TeamID:       "team_1",
				AssigneeID:   "user_1",
				CreatorID:    "user_2",
				UpdatedSince: "2026-09-01T00:00:00Z",
			},
			want: map[string]any{
				"team":      eq("team_1"),
				"assignee":  eq("user_1"),
				"creator":   eq("user_2"),
				"updatedAt": map[string]any{"gte": "2026-09-01T00:00:00Z"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, calls := mockGraphQL(t, func(gqlCall) any {
				return map[string]any{"issues": conn([]any{
					issueNode("issue_1", "ENG-1", "First"),
				}, false, "")}
			})
			svc := newTestService(srv)

			issues, err := svc.ListIssues(context.Background(), tt.filter)
			require.NoError(t, err)
			require.Len(t, issues, 1)
			require.Len(t, *calls, 1)
			// One top-level issues query carrying the filter as a variable.
			require.Contains(t, (*calls)[0].Query, "issues(filter: $filter")
			require.NotContains(t, (*calls)[0].Query, "team(id:")
			require.Equal(t, tt.want, (*calls)[0].Variables["filter"])
		})
	}
}

func TestListIssuesZeroFilterOmitsFilterVariable(t *testing.T) {
	srv, calls := mockGraphQL(t, func(gqlCall) any {
		return map[string]any{"issues": conn([]any{
			issueNode("issue_1", "ENG-1", "First"),
		}, false, "")}
	})
	svc := newTestService(srv)

	_, err := svc.ListIssues(context.Background(), IssueFilter{})
	require.NoError(t, err)
	require.Len(t, *calls, 1)
	// The unfiltered wire shape is preserved: no filter variable at all,
	// not an empty object.
	require.NotContains(t, (*calls)[0].Variables, "filter")
	require.Contains(t, (*calls)[0].Query, "issues(filter: $filter")
}

// commentNode returns a mock comment node.
func commentNode(id, body string) map[string]any {
	return map[string]any{
		"id": id, "body": body,
		"createdAt": "2026-08-01T12:00:00.000Z", "updatedAt": "2026-08-01T12:00:00.000Z",
		"parent": map[string]any{"id": "comment_0"},
		"user":   map[string]any{"id": "user_1", "name": "Ada"},
	}
}

func TestListCommentsFollowsCursorAcrossTwoPages(t *testing.T) {
	srv, calls := mockGraphQL(t, func(call gqlCall) any {
		if _, paged := call.Variables["after"]; !paged {
			return map[string]any{"issue": map[string]any{"comments": conn([]any{
				commentNode("comment_1", "First comment"),
			}, true, "cursor-1")}}
		}
		return map[string]any{"issue": map[string]any{"comments": conn([]any{
			commentNode("comment_2", "Second comment"),
		}, false, "")}}
	})
	svc := newTestService(srv)

	comments, err := svc.ListComments(context.Background(), "issue_1")
	require.NoError(t, err)
	require.Len(t, comments, 2)
	require.Equal(t, "First comment", comments[0].Body)
	require.Equal(t, "Ada", comments[0].User.Name)
	require.Equal(t, "comment_0", comments[0].Parent.ID)

	require.Len(t, *calls, 2)
	require.Equal(t, "issue_1", (*calls)[0].Variables["id"])
	require.Equal(t, "cursor-1", (*calls)[1].Variables["after"])
	// The selection set is exactly the agreed comment fields, paginated on
	// the issue.comments connection.
	require.Contains(t, (*calls)[0].Query, "comments(first: $first, after: $after)")
	require.Contains(t, (*calls)[0].Query, "id body createdAt updatedAt parent { id } user { id name }")
}

func TestListCommentsIssueNullSurfacesDigError(t *testing.T) {
	srv, _ := mockGraphQL(t, func(gqlCall) any {
		return map[string]any{"issue": nil}
	})
	svc := newTestService(srv)

	// A null issue leaves no comments to walk into; dig names the null
	// intermediate segment.
	_, err := svc.ListComments(context.Background(), "issue_999")
	require.ErrorContains(t, err, `linear response has null "issue"`)
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
		ProjectID:   "proj_1",
	})
	require.NoError(t, err)
	require.Equal(t, "ENG-4", issue.Identifier)

	input, ok := (*calls)[0].Variables["input"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "team_1", input["teamId"])
	require.Equal(t, "Fourth", input["title"])
	require.Equal(t, "details", input["description"])
	require.Equal(t, "state_1", input["stateId"])
	require.Equal(t, "proj_1", input["projectId"])
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

func TestUpdateIssueSendsProjectID(t *testing.T) {
	srv, calls := mockGraphQL(t, func(gqlCall) any {
		return map[string]any{"issueUpdate": map[string]any{
			"success": true, "issue": issueNode("issue_1", "ENG-1", "Fix login redirect"),
		}}
	})
	svc := newTestService(srv)

	_, err := svc.UpdateIssue(context.Background(), "ENG-1", UpdateIssueInput{ProjectID: "proj_1"})
	require.NoError(t, err)

	input, ok := (*calls)[0].Variables["input"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, map[string]any{"projectId": "proj_1"}, input)
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

func TestMutationDecodeErrorsNameTheFailingHalf(t *testing.T) {
	// A malformed payload must say which half failed to decode — the
	// success flag or the node — so the two failures are distinguishable.
	tests := []struct {
		name    string
		payload map[string]any
		want    string
	}{
		{"success half", map[string]any{"success": "not-a-bool", "issue": nil}, "decoding issueCreate success"},
		{"node half", map[string]any{"success": true, "issue": "not-an-object"}, "decoding issueCreate issue"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, _ := mockGraphQL(t, func(gqlCall) any {
				return map[string]any{"issueCreate": tt.payload}
			})
			svc := newTestService(srv)

			_, err := svc.CreateIssue(context.Background(), CreateIssueInput{TeamID: "team_1", Title: "X"})
			require.ErrorContains(t, err, tt.want)
		})
	}
}
