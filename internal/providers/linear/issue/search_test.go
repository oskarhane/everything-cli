package issue

import (
	"context"
	"fmt"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

// fakeSearch is the hermetic service.SearchService double: it serves seeded
// issues and records the query it was asked for.
type fakeSearch struct {
	issues []service.Issue
	err    error
	query  string
}

func (f *fakeSearch) SearchIssues(_ context.Context, query string) ([]service.Issue, error) {
	f.query = query
	if f.err != nil {
		return nil, f.err
	}
	return f.issues, nil
}

// fakeNewSearchSvc hands out svc so the leaf runs hermetically.
func fakeNewSearchSvc(svc *fakeSearch) service.Dialer[service.SearchService] {
	return func(context.Context) (service.SearchService, error) { return svc, nil }
}

// newSearchTestCmd builds the search leaf against a fake service, ready to
// execute.
func newSearchTestCmd(svc *fakeSearch, format string) *cobra.Command {
	return newSearchCmd(cmdtest.NewTestConfig(format), fakeNewSearchSvc(svc))
}

func TestSearchJSON(t *testing.T) {
	svc := &fakeSearch{issues: []service.Issue{seedIssue()}}
	out := cmdtest.RunCmd(t, newSearchTestCmd(svc, "json"), "--query", "login redirect")

	got := cmdtest.DecodeJSON(t, out)
	m, ok := got.(map[string]any)
	require.True(t, ok, "one match renders as a single object: %v", got)
	require.Equal(t, "ENG-1", m["identifier"])
	require.Equal(t, "login redirect", svc.query)
}

func TestSearchTable(t *testing.T) {
	svc := &fakeSearch{issues: []service.Issue{seedIssue()}}
	out := cmdtest.RunCmd(t, newSearchTestCmd(svc, "table"), "--query", "login redirect")

	// go-pretty StyleLight upper-cases header cells.
	require.Contains(t, out, "IDENTIFIER")
	require.Contains(t, out, "UPDATED_AT")
	require.Contains(t, out, "ENG-1")
}

func TestSearchToon(t *testing.T) {
	svc := &fakeSearch{issues: []service.Issue{seedIssue()}}
	out := cmdtest.RunCmd(t, newSearchTestCmd(svc, "toon"), "--query", "login redirect")
	require.Contains(t, out, "identifier: ENG-1")
}

// TestSearchEmpty pins the empty-result contract: no matches render as an
// empty list in every format, not an error or null.
func TestSearchEmpty(t *testing.T) {
	t.Run("json", func(t *testing.T) {
		svc := &fakeSearch{}
		out := cmdtest.RunCmd(t, newSearchTestCmd(svc, "json"), "--query", "nothing matches")
		require.Equal(t, []any{}, cmdtest.DecodeJSON(t, out))
	})
	t.Run("table", func(t *testing.T) {
		svc := &fakeSearch{}
		out := cmdtest.RunCmd(t, newSearchTestCmd(svc, "table"), "--query", "nothing matches")
		require.Contains(t, out, "IDENTIFIER")
		require.NotContains(t, out, "ENG-1")
	})
	t.Run("toon", func(t *testing.T) {
		svc := &fakeSearch{}
		out := cmdtest.RunCmd(t, newSearchTestCmd(svc, "toon"), "--query", "nothing matches")
		require.Contains(t, out, "[#0]", "empty toon list marker")
		require.NotContains(t, out, "ENG-1")
	})
}

// TestSearchQueryRequired pins the required --query flag: the service is
// never dialed without it.
func TestSearchQueryRequired(t *testing.T) {
	svc := &fakeSearch{issues: []service.Issue{seedIssue()}}
	dialed := false
	newSvc := func(context.Context) (service.SearchService, error) {
		dialed = true
		return svc, nil
	}
	cmd := newSearchCmd(cmdtest.NewTestConfig("json"), newSvc)

	_, err := cmdtest.RunCmdErr(t, cmd)

	require.ErrorContains(t, err, "query")
	require.False(t, dialed, "a missing --query must fail before any SearchService call")
}

// TestSearchServiceError pins error propagation: a search failure surfaces
// and nothing renders.
func TestSearchServiceError(t *testing.T) {
	svc := &fakeSearch{err: fmt.Errorf("boom")}
	_, err := cmdtest.RunCmdErr(t, newSearchTestCmd(svc, "json"), "--query", "x")
	require.ErrorContains(t, err, "boom")
}
