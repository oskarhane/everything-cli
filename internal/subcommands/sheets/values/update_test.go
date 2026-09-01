package values

import (
	"errors"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/google-cli/internal/subcommands/cmdtest"
)

func TestUpdateJSONFlagValues(t *testing.T) {
	svc := &fakeValuesService{updateUpdated: "Sheet1!A1:B2", updatedCells: 4}
	out := cmdtest.RunCmd(t, newLeafCmd(newUpdateCmd, svc, "json"), seedSpreadsheetID,
		"--range", "Sheet1!A1:B2", "--values", `[[1,"a"],[2,"b"]]`)

	detail, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok, "expected one JSON object, got: %s", out)
	keys := cmdtest.JSONKeys(t, detail)
	require.ElementsMatch(t, updateFields, keys)
	cmdtest.RequireSnakeCase(t, keys)
	require.Equal(t, "Sheet1!A1:B2", detail["updated_range"])
	require.EqualValues(t, 4, detail["updated_cells"])
}

func TestUpdateRoundTripsTyped2DValues(t *testing.T) {
	svc := &fakeValuesService{}
	cmdtest.RunCmd(t, newLeafCmd(newUpdateCmd, svc, "json"), seedSpreadsheetID,
		"--range", "Sheet1!A1:B2", "--values", `[["=SUM(A1:A2)",false]]`)

	require.Equal(t, seedSpreadsheetID, svc.updateID)
	require.Equal(t, "Sheet1!A1:B2", svc.updateRange)
	require.Equal(t, [][]any{{"=SUM(A1:A2)", false}}, svc.updated)
	require.Equal(t, "USER_ENTERED", svc.updateOption, "default input option")
}

func TestUpdateRejectsInvalidInputOption(t *testing.T) {
	svc := &fakeValuesService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newUpdateCmd, svc, "json"), seedSpreadsheetID,
		"--range", "Sheet1!A1:B2", "--values", `[[1]]`, "--input-option", "NONSENSE")

	require.Contains(t, err.Error(), `invalid --input-option "NONSENSE"`)
	require.Contains(t, err.Error(), "RAW or USER_ENTERED")
	require.Empty(t, svc.updateID)
}

func TestUpdateRejectsBothValueSources(t *testing.T) {
	svc := &fakeValuesService{}
	cmd := newLeafCmdWithFs(newUpdateCmd, svc, "json", func(fs afero.Fs) {
		writeFile(t, fs, "/tmp/cells.json", `[[1]]`)
	})
	_, err := cmdtest.RunCmdErr(t, cmd, seedSpreadsheetID,
		"--range", "Sheet1!A1:B2", "--values", `[[1]]`, "--values-file", "/tmp/cells.json")

	require.Contains(t, err.Error(), "one source only")
	require.Empty(t, svc.updateID)
}

func TestUpdateValuesFileJSON(t *testing.T) {
	svc := &fakeValuesService{}
	cmd := newLeafCmdWithFs(newUpdateCmd, svc, "json", func(fs afero.Fs) {
		writeFile(t, fs, "/tmp/cells.json", `[[true,"x"]]`)
	})
	cmdtest.RunCmd(t, cmd, seedSpreadsheetID, "--range", "Sheet1!A1:B2", "--values-file", "/tmp/cells.json")

	require.Equal(t, [][]any{{true, "x"}}, svc.updated)
	require.Equal(t, "Sheet1!A1:B2", svc.updateRange)
}

func TestUpdateValuesFileCSV(t *testing.T) {
	svc := &fakeValuesService{}
	cmd := newLeafCmdWithFs(newUpdateCmd, svc, "json", func(fs afero.Fs) {
		writeFile(t, fs, "/tmp/cells.csv", "1,a\n")
	})
	cmdtest.RunCmd(t, cmd, seedSpreadsheetID, "--range", "Sheet1!A1:B2", "--values-file", "/tmp/cells.csv")

	require.Equal(t, [][]any{{"1", "a"}}, svc.updated)
}

func TestUpdateValuesFileTSV(t *testing.T) {
	svc := &fakeValuesService{}
	cmd := newLeafCmdWithFs(newUpdateCmd, svc, "json", func(fs afero.Fs) {
		writeFile(t, fs, "/tmp/cells.tsv", "1\ta\n")
	})
	cmdtest.RunCmd(t, cmd, seedSpreadsheetID, "--range", "Sheet1!A1:B2", "--values-file", "/tmp/cells.tsv")

	require.Equal(t, [][]any{{"1", "a"}}, svc.updated)
}

func TestUpdateMissingValues(t *testing.T) {
	svc := &fakeValuesService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newUpdateCmd, svc, "json"), seedSpreadsheetID, "--range", "Sheet1!A1:B2")

	require.Contains(t, err.Error(), "no values given")
	require.Empty(t, svc.updateID)
}

func TestUpdateRequiresRange(t *testing.T) {
	svc := &fakeValuesService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newUpdateCmd, svc, "json"), seedSpreadsheetID, "--values", `[[1]]`)

	require.Contains(t, err.Error(), `required flag(s) "range" not set`)
}

func TestUpdatePropagatesAPIError(t *testing.T) {
	svc := &fakeValuesService{updateErr: errors.New("googleapi: Error 400")}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newUpdateCmd, svc, "json"), seedSpreadsheetID,
		"--range", "Sheet1!A1:B2", "--values", `[[1]]`)

	require.Contains(t, err.Error(), "googleapi: Error 400")
}

func TestUpdateTableUpperCasesHeaders(t *testing.T) {
	svc := &fakeValuesService{updateUpdated: "Sheet1!A1", updatedCells: 1}
	out := cmdtest.RunCmd(t, newLeafCmd(newUpdateCmd, svc, "table"), seedSpreadsheetID,
		"--range", "Sheet1!A1", "--values", `[[1]]`)

	for _, header := range []string{"UPDATED_RANGE", "UPDATED_CELLS"} {
		require.Contains(t, out, header)
	}
}
