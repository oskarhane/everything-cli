package slides

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/google-cli/internal/subcommands/cmdtest"
)

func TestGetJSON(t *testing.T) {
	svc := &fakeSlideService{shapes: seedShapes()}
	out := cmdtest.RunCmd(t, newSlideLeafCmd(newGetCmd, svc, "json"), "pres_1")

	rows, ok := cmdtest.DecodeJSON(t, out).([]any)
	require.True(t, ok, "expected a JSON array, got: %s", out)
	require.Len(t, rows, 4)

	first, ok := rows[0].(map[string]any)
	require.True(t, ok)
	keys := cmdtest.JSONKeys(t, first)
	require.ElementsMatch(t, []string{"slide", "shape_id", "text"}, keys)
	cmdtest.RequireSnakeCase(t, keys)
	require.EqualValues(t, 1, first["slide"])
	require.Equal(t, "shape_title_1", first["shape_id"])
	require.Equal(t, "Quarterly Review", first["text"])
}

func TestGetTable(t *testing.T) {
	svc := &fakeSlideService{shapes: seedShapes()}
	out := cmdtest.RunCmd(t, newSlideLeafCmd(newGetCmd, svc, "table"), "pres_1")

	// go-pretty StyleLight upper-cases the headers.
	for _, header := range []string{"SLIDE", "SHAPE_ID", "TEXT"} {
		require.Contains(t, out, header)
	}
	require.Contains(t, out, "shape_title_1")
	require.Contains(t, out, "Revenue is up 12% QoQ")
}

func TestGetSkipsTextlessShapes(t *testing.T) {
	// The seam only returns text-bearing shapes, but an empty presentation
	// must render as an empty array, not null.
	svc := &fakeSlideService{}
	out := cmdtest.RunCmd(t, newSlideLeafCmd(newGetCmd, svc, "json"), "pres_1")

	require.Equal(t, []any{}, cmdtest.DecodeJSON(t, out))
}

func TestGetSlideFilter(t *testing.T) {
	svc := &fakeSlideService{shapes: seedShapes()}
	out := cmdtest.RunCmd(t, newSlideLeafCmd(newGetCmd, svc, "json"), "pres_1", "--slide", "2")

	rows, ok := cmdtest.DecodeJSON(t, out).([]any)
	require.True(t, ok, "expected a JSON array, got: %s", out)
	require.Len(t, rows, 2)
	for _, r := range rows {
		row := r.(map[string]any)
		require.EqualValues(t, 2, row["slide"])
	}
}

func TestGetSlideFilterNoMatch(t *testing.T) {
	svc := &fakeSlideService{shapes: seedShapes()}
	out := cmdtest.RunCmd(t, newSlideLeafCmd(newGetCmd, svc, "json"), "pres_1", "--slide", "9")

	require.Equal(t, []any{}, cmdtest.DecodeJSON(t, out))
}

func TestGetPassesPresentationID(t *testing.T) {
	svc := &fakeSlideService{shapes: seedShapes()}
	cmdtest.RunCmd(t, newSlideLeafCmd(newGetCmd, svc, "json"), "pres_1")

	require.Equal(t, "pres_1", svc.getID)
}

func TestGetPropagatesAPIError(t *testing.T) {
	svc := &fakeSlideService{err: errors.New("googleapi: Error 404")}
	_, err := cmdtest.RunCmdErr(t, newSlideLeafCmd(newGetCmd, svc, "json"), "pres_1")

	require.Contains(t, err.Error(), "googleapi: Error 404")
}
