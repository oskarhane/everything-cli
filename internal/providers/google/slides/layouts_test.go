package slides

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestLayoutsJSON(t *testing.T) {
	svc := &fakeSlideService{layouts: seedLayouts()}
	out := cmdtest.RunCmd(t, newSlideLeafCmd(newLayoutsCmd, svc, "json"), "pres_1")

	rows, ok := cmdtest.DecodeJSON(t, out).([]any)
	require.True(t, ok, "expected a JSON array, got: %s", out)
	require.Len(t, rows, 2)
	require.Equal(t, "pres_1", svc.layoutsID)

	first, ok := rows[0].(map[string]any)
	require.True(t, ok)
	keys := cmdtest.JSONKeys(t, first)
	require.ElementsMatch(t, []string{"object_id", "name"}, keys)
	cmdtest.RequireSnakeCase(t, keys)
	require.Equal(t, "layout_title_only", first["object_id"])
	require.Equal(t, "TITLE_ONLY", first["name"])
}

func TestLayoutsTable(t *testing.T) {
	svc := &fakeSlideService{layouts: seedLayouts()}
	out := cmdtest.RunCmd(t, newSlideLeafCmd(newLayoutsCmd, svc, "table"), "pres_1")

	// go-pretty StyleLight upper-cases the headers.
	for _, header := range []string{"OBJECT_ID", "NAME"} {
		require.Contains(t, out, header)
	}
	require.Contains(t, out, "layout_title_only")
	require.Contains(t, out, "TITLE_ONLY")
}

func TestLayoutsToon(t *testing.T) {
	svc := &fakeSlideService{layouts: seedLayouts()}
	out := cmdtest.RunCmd(t, newSlideLeafCmd(newLayoutsCmd, svc, "toon"), "pres_1")

	// TOON round-trips through JSON, so keys alphabetize: name, object_id.
	require.Contains(t, out, "{name,object_id}:")
	require.Contains(t, out, "TITLE_ONLY,layout_title_only")
	require.Contains(t, out, "TITLE_AND_BODY,layout_title_and_body")
}

func TestLayoutsEmptyRendersZeroRows(t *testing.T) {
	svc := &fakeSlideService{}
	out := cmdtest.RunCmd(t, newSlideLeafCmd(newLayoutsCmd, svc, "json"), "pres_1")

	require.Equal(t, []any{}, cmdtest.DecodeJSON(t, out))
}

func TestLayoutsPropagatesAPIError(t *testing.T) {
	svc := &fakeSlideService{err: errors.New("googleapi: Error 404")}
	_, err := cmdtest.RunCmdErr(t, newSlideLeafCmd(newLayoutsCmd, svc, "json"), "pres_1")

	require.Contains(t, err.Error(), "googleapi: Error 404")
}
