package relation

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestListRequiresIssue(t *testing.T) {
	svc := &fakeService{relations: seedRelations()}
	_, err := cmdtest.RunCmdErr(t, listCmd(svc, "json"))

	require.Error(t, err, "cobra rejects a missing --issue before RunE")
	require.False(t, svc.dialed, "the service is never dialed without --issue")
}

func TestListPassesIssueFlag(t *testing.T) {
	svc := &fakeService{relations: seedRelations()}
	cmdtest.RunCmd(t, listCmd(svc, "json"), "--issue", "ENG-1")

	require.Equal(t, "ENG-1", svc.listIssueID)
}

func TestListJSONCoversBothDirections(t *testing.T) {
	svc := &fakeService{relations: seedRelations()}
	out := cmdtest.RunCmd(t, listCmd(svc, "json"), "--issue", "ENG-1")

	got, ok := cmdtest.DecodeJSON(t, out).([]any)
	require.True(t, ok, "two relations render as an array: %v", out)
	require.Len(t, got, 2)

	outgoing, ok := got[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "rel_1", outgoing["id"])
	require.Equal(t, "blocks", outgoing["type"])
	require.Equal(t, "outgoing", outgoing["direction"])
	require.Equal(t, "ENG-2", outgoing["identifier"])
	require.Equal(t, "Second", outgoing["title"])
	cmdtest.RequireSnakeCase(t, cmdtest.JSONKeys(t, outgoing))

	incoming, ok := got[1].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "rel_2", incoming["id"])
	require.Equal(t, "blocked-by", incoming["type"], "an incoming blocks relation reads blocked-by")
	require.Equal(t, "incoming", incoming["direction"])
	require.Equal(t, "ENG-3", incoming["identifier"], "the other issue is the blocker")
	require.Equal(t, "Third", incoming["title"])
}

func TestListSingleRelationCollapsesToObject(t *testing.T) {
	svc := &fakeService{relations: seedRelations()[:1]}
	out := cmdtest.RunCmd(t, listCmd(svc, "json"), "--issue", "ENG-1")

	m, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok, "one relation renders as a single object: %v", out)
	require.Equal(t, "rel_1", m["id"])
}

func TestListEmptyRendersEmptyArray(t *testing.T) {
	svc := &fakeService{}
	out := cmdtest.RunCmd(t, listCmd(svc, "json"), "--issue", "ENG-1")

	got, ok := cmdtest.DecodeJSON(t, out).([]any)
	require.True(t, ok, "no relations render as [], not null: %v", out)
	require.Empty(t, got)
}

func TestListTable(t *testing.T) {
	svc := &fakeService{relations: seedRelations()}
	out := cmdtest.RunCmd(t, listCmd(svc, "table"), "--issue", "ENG-1")

	// go-pretty StyleLight upper-cases header cells.
	require.Contains(t, out, "ID")
	require.Contains(t, out, "TYPE")
	require.Contains(t, out, "DIRECTION")
	require.Contains(t, out, "IDENTIFIER")
	require.Contains(t, out, "TITLE")
	require.Contains(t, out, "blocks")
	require.Contains(t, out, "blocked-by")
	require.Contains(t, out, "ENG-2")
	require.Contains(t, out, "ENG-3")
}

func TestListToon(t *testing.T) {
	svc := &fakeService{relations: seedRelations()}
	out := cmdtest.RunCmd(t, listCmd(svc, "toon"), "--issue", "ENG-1")

	// A multi-row list renders in TOON's tabular array form.
	require.Contains(t, out, "{direction,id,identifier,title,type}:")
	require.Contains(t, out, "blocked-by")
}

func TestListError(t *testing.T) {
	svc := &fakeService{err: errors.New(`issue "ENG-999" not found`)}
	_, err := cmdtest.RunCmdErr(t, listCmd(svc, "json"), "--issue", "ENG-999")

	require.ErrorContains(t, err, `issue "ENG-999" not found`)
}
