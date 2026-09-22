package api

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

// TestMain neutralizes format auto-detection so the host's harness env and
// TTY cannot flip output expectations.
func TestMain(m *testing.M) {
	output.IsAgent = func() bool { return false }
	output.StdoutIsTerminal = func() bool { return false }
	os.Exit(m.Run())
}

// fakeService is the hermetic service.GraphQLService double.
type fakeService struct {
	data     json.RawMessage
	err      error
	called   bool
	gotQuery string
	gotVars  map[string]any
}

func (f *fakeService) ExecGraphQL(_ context.Context, query string, variables map[string]any) (json.RawMessage, error) {
	f.called = true
	f.gotQuery = query
	f.gotVars = variables
	return f.data, f.err
}

func newCmdForTest(cfg *app.Config, svc *fakeService) *cobra.Command {
	return NewCmd(cfg, func(context.Context) (service.GraphQLService, error) { return svc, nil })
}

func TestAPIPrintsRawDataDocument(t *testing.T) {
	svc := &fakeService{data: json.RawMessage(`{"viewer":{"id":"user_1","name":"Ada"}}`)}
	out := cmdtest.RunCmd(t, newCmdForTest(cmdtest.NewTestConfig("json"), svc), "{ viewer { id name } }")

	require.JSONEq(t, `{"viewer":{"id":"user_1","name":"Ada"}}`, out)
	require.True(t, svc.called)
	require.Equal(t, "{ viewer { id name } }", svc.gotQuery)
	require.Nil(t, svc.gotVars)
}

func TestAPIInlineVariables(t *testing.T) {
	svc := &fakeService{data: json.RawMessage(`{"issue":{"id":"issue_1"}}`)}
	out := cmdtest.RunCmd(t, newCmdForTest(cmdtest.NewTestConfig("json"), svc),
		"query($id: String!) { issue(id: $id) { id } }",
		"--variables", `{"id": "ENG-1", "first": 5}`)

	require.JSONEq(t, `{"issue":{"id":"issue_1"}}`, out)
	require.Equal(t, map[string]any{"id": "ENG-1", "first": float64(5)}, svc.gotVars)
}

func TestAPIVariablesFromFile(t *testing.T) {
	cfg := cmdtest.NewTestConfig("json")
	require.NoError(t, afero.WriteFile(cfg.Fs, "vars.json", []byte(`{"id": "ENG-1"}`), 0o600))
	svc := &fakeService{data: json.RawMessage(`{"issue":{"id":"issue_1"}}`)}

	out := cmdtest.RunCmd(t, newCmdForTest(cfg, svc),
		"query($id: String!) { issue(id: $id) { id } }", "--variables", "@vars.json")

	require.JSONEq(t, `{"issue":{"id":"issue_1"}}`, out)
	require.Equal(t, map[string]any{"id": "ENG-1"}, svc.gotVars)
}

func TestAPIGraphQLErrorExitsNonZero(t *testing.T) {
	svc := &fakeService{err: errors.New(`linear API error: Field "nope" doesn't exist (GRAPHQL_VALIDATION_FAILED)`)}
	_, err := cmdtest.RunCmdErr(t, newCmdForTest(cmdtest.NewTestConfig("json"), svc), "{ nope }")
	require.ErrorContains(t, err, `Field "nope" doesn't exist`)
}

func TestAPIMalformedInlineVariablesErrorBeforeCall(t *testing.T) {
	svc := &fakeService{}
	_, err := cmdtest.RunCmdErr(t, newCmdForTest(cmdtest.NewTestConfig("json"), svc),
		"{ viewer { id } }", "--variables", `{"id": `)
	require.ErrorContains(t, err, "parsing --variables")
	require.False(t, svc.called, "malformed variables must fail before any API call")
}

func TestAPIMalformedVariablesFileErrorBeforeCall(t *testing.T) {
	cfg := cmdtest.NewTestConfig("json")
	require.NoError(t, afero.WriteFile(cfg.Fs, "vars.json", []byte(`not json`), 0o600))
	svc := &fakeService{}

	_, err := cmdtest.RunCmdErr(t, newCmdForTest(cfg, svc), "{ viewer { id } }", "--variables", "@vars.json")
	require.ErrorContains(t, err, "parsing --variables")
	require.False(t, svc.called)
}

func TestAPIMissingVariablesFile(t *testing.T) {
	svc := &fakeService{}
	_, err := cmdtest.RunCmdErr(t, newCmdForTest(cmdtest.NewTestConfig("json"), svc),
		"{ viewer { id } }", "--variables", "@nope.json")
	require.ErrorContains(t, err, "reading variables file")
	require.False(t, svc.called)
}
