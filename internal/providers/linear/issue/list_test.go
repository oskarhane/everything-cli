package issue

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

// fakeViewer is the hermetic service.ViewerService double: it serves one
// seeded viewer and counts GetViewer calls so tests can pin the one-call
// me resolution.
type fakeViewer struct {
	viewer service.Viewer
	err    error
	calls  int
}

func (f *fakeViewer) GetViewer(_ context.Context) (service.Viewer, error) {
	f.calls++
	if f.err != nil {
		return service.Viewer{}, f.err
	}
	return f.viewer, nil
}

// fakeViewerSvc hands out v so the leaf resolves me hermetically.
func fakeViewerSvc(v *fakeViewer) service.Dialer[service.ViewerService] {
	return func(context.Context) (service.ViewerService, error) { return v, nil }
}

// unusedViewerSvc fails the run if the leaf dials the viewer service when
// no flag asks for me.
func unusedViewerSvc() service.Dialer[service.ViewerService] {
	return func(context.Context) (service.ViewerService, error) {
		return nil, fmt.Errorf("viewer service dialed but no flag asked for me")
	}
}

// newListTestCmd builds the list leaf directly: newListCmd takes both the
// issue dialer and the viewer dialer, which newLeafCmd does not carry.
func newListTestCmd(svc *fakeService, viewerSvc service.Dialer[service.ViewerService], format string) *cobra.Command {
	return newListCmd(cmdtest.NewTestConfig(format), fakeNewSvc(svc), viewerSvc)
}

// pinNow pins the clock seam for one test.
func pinNow(t *testing.T, now time.Time) {
	t.Helper()
	old := nowFunc
	nowFunc = func() time.Time { return now }
	t.Cleanup(func() { nowFunc = old })
}

func TestListJSON(t *testing.T) {
	svc := &fakeService{issues: []service.Issue{seedIssue()}}
	out := cmdtest.RunCmd(t, newListTestCmd(svc, unusedViewerSvc(), "json"))

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
	require.Equal(t, service.IssueFilter{}, svc.listFilter)
}

func TestListTable(t *testing.T) {
	svc := &fakeService{issues: []service.Issue{seedIssue()}}
	out := cmdtest.RunCmd(t, newListTestCmd(svc, unusedViewerSvc(), "table"))

	// go-pretty StyleLight upper-cases header cells.
	require.Contains(t, out, "IDENTIFIER")
	require.Contains(t, out, "UPDATED_AT")
	require.Contains(t, out, "ENG-1")
	require.Contains(t, out, "In Progress")
}

func TestListToon(t *testing.T) {
	svc := &fakeService{issues: []service.Issue{seedIssue()}}
	out := cmdtest.RunCmd(t, newListTestCmd(svc, unusedViewerSvc(), "toon"))
	require.Contains(t, out, "identifier: ENG-1")
}

// TestListFilterFlags maps every scope flag onto the single server-side
// filter passed to ListIssues.
func TestListFilterFlags(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		filter service.IssueFilter
	}{
		{
			name:   "team",
			args:   []string{"--team", "team_1"},
			filter: service.IssueFilter{TeamID: "team_1"},
		},
		{
			name:   "assignee uuid",
			args:   []string{"--assignee", "user_1"},
			filter: service.IssueFilter{AssigneeID: "user_1"},
		},
		{
			name:   "created-by uuid",
			args:   []string{"--created-by", "user_2"},
			filter: service.IssueFilter{CreatorID: "user_2"},
		},
		{
			name: "all composed",
			args: []string{"--team", "team_1", "--assignee", "user_1", "--created-by", "user_2"},
			filter: service.IssueFilter{
				TeamID:     "team_1",
				AssigneeID: "user_1",
				CreatorID:  "user_2",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeService{issues: []service.Issue{seedIssue()}}
			cmdtest.RunCmd(t, newListTestCmd(svc, unusedViewerSvc(), "json"), tt.args...)
			require.Equal(t, tt.filter, svc.listFilter)
		})
	}
}

// TestListMeResolvesOnce pins the me resolution: with both user flags set
// to me in one run, GetViewer is called exactly once and both filter
// fields carry the viewer's ID.
func TestListMeResolvesOnce(t *testing.T) {
	viewer := &fakeViewer{viewer: service.Viewer{ID: "user_9", Name: "Ada", Email: "ada@example.com"}}
	svc := &fakeService{issues: []service.Issue{seedIssue()}}
	cmd := newListCmd(cmdtest.NewTestConfig("json"), fakeNewSvc(svc), fakeViewerSvc(viewer))

	cmdtest.RunCmd(t, cmd, "--assignee", "me", "--created-by", "me")

	require.Equal(t, 1, viewer.calls, "one GetViewer call per run, shared by both flags")
	require.Equal(t, service.IssueFilter{AssigneeID: "user_9", CreatorID: "user_9"}, svc.listFilter)
}

// TestListMeMixedWithUUID pins that a literal UUID passes through while
// me still resolves — and only for the flag that asked.
func TestListMeMixedWithUUID(t *testing.T) {
	viewer := &fakeViewer{viewer: service.Viewer{ID: "user_9", Name: "Ada", Email: "ada@example.com"}}
	svc := &fakeService{issues: []service.Issue{seedIssue()}}
	cmd := newListCmd(cmdtest.NewTestConfig("json"), fakeNewSvc(svc), fakeViewerSvc(viewer))

	cmdtest.RunCmd(t, cmd, "--assignee", "user_1", "--created-by", "me")

	require.Equal(t, 1, viewer.calls)
	require.Equal(t, service.IssueFilter{AssigneeID: "user_1", CreatorID: "user_9"}, svc.listFilter)
}

// TestListUpdatedSince parses the accepted --updated-since forms against a
// pinned clock.
func TestListUpdatedSince(t *testing.T) {
	pinNow(t, time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC))
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "relative -1d", value: "-1d", want: "2026-09-08T12:00:00Z"},
		{name: "relative now", value: "now", want: "2026-09-09T12:00:00Z"},
		{name: "absolute RFC3339", value: "2026-09-01T00:00:00Z", want: "2026-09-01T00:00:00Z"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeService{issues: []service.Issue{seedIssue()}}
			cmdtest.RunCmd(t, newListTestCmd(svc, unusedViewerSvc(), "json"), "--updated-since", tt.value)
			require.Equal(t, service.IssueFilter{UpdatedSince: tt.want}, svc.listFilter)
		})
	}
}

// TestListUpdatedSinceInvalid pins the fail-fast path: a bad value errors
// naming --updated-since and the issue service is never dialed.
func TestListUpdatedSinceInvalid(t *testing.T) {
	svc := &fakeService{issues: []service.Issue{seedIssue()}}
	dialed := false
	newSvc := func(context.Context) (service.IssueService, error) {
		dialed = true
		return svc, nil
	}
	cmd := newListCmd(cmdtest.NewTestConfig("json"), newSvc, unusedViewerSvc())

	_, err := cmdtest.RunCmdErr(t, cmd, "--updated-since", "nonsense")

	require.ErrorContains(t, err, "--updated-since")
	require.False(t, dialed, "parse errors must return before any IssueService call")
	require.Equal(t, service.IssueFilter{}, svc.listFilter)
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
	out := cmdtest.RunCmd(t, newListCmd(cfg, serverSvc(srv), unusedViewerSvc()))

	got := cmdtest.DecodeJSON(t, out)
	arr, ok := got.([]any)
	require.True(t, ok, "two pages render as an array: %v", got)
	require.Len(t, arr, 2)
	require.Len(t, calls, 2)
	require.Equal(t, "cursor-1", calls[1]["after"])
}
