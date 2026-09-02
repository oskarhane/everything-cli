package values

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestAppendJSONFlagValues(t *testing.T) {
	svc := &fakeValuesService{appendUpdated: "Sheet1!A3:D4", appendRows: 2, appendCols: 4}
	out := cmdtest.RunCmd(t, newLeafCmd(newAppendCmd, svc, "json"), seedSpreadsheetID,
		"--range", "Sheet1!A1:D", "--values", `[[1,"a",true],[2,"b",false]]`)

	detail, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok, "expected one JSON object, got: %s", out)
	keys := cmdtest.JSONKeys(t, detail)
	require.ElementsMatch(t, appendFields, keys)
	cmdtest.RequireSnakeCase(t, keys)
	require.Equal(t, "Sheet1!A3:D4", detail["updated_range"])
	require.EqualValues(t, 2, detail["updated_rows"])
	require.EqualValues(t, 4, detail["updated_cols"])
}

func TestAppendRoundTripsTyped2DValues(t *testing.T) {
	svc := &fakeValuesService{}
	cmdtest.RunCmd(t, newLeafCmd(newAppendCmd, svc, "json"), seedSpreadsheetID,
		"--range", "Sheet1!A1:D", "--values", `[[1,"a",true],[2.5,"b",false]]`)

	require.Equal(t, seedSpreadsheetID, svc.appendID)
	require.Equal(t, "Sheet1!A1:D", svc.appendRange)
	require.Equal(t, [][]any{
		{json.Number("1"), "a", true},
		{json.Number("2.5"), "b", false},
	}, svc.appended)
	require.Equal(t, "USER_ENTERED", svc.appendOption, "default input option")
}

func TestAppendRecordsExplicitInputOption(t *testing.T) {
	svc := &fakeValuesService{}
	cmdtest.RunCmd(t, newLeafCmd(newAppendCmd, svc, "json"), seedSpreadsheetID,
		"--range", "Sheet1!A1:D", "--values", `[["=SUM(A1:A2)"]]`, "--input-option", "RAW")

	require.Equal(t, "RAW", svc.appendOption)
}

func TestAppendRejectsInvalidInputOption(t *testing.T) {
	svc := &fakeValuesService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newAppendCmd, svc, "json"), seedSpreadsheetID,
		"--range", "Sheet1!A1:D", "--values", `[[1]]`, "--input-option", "UNFORMATTED")

	require.Contains(t, err.Error(), `invalid --input-option "UNFORMATTED"`)
	require.Contains(t, err.Error(), "RAW or USER_ENTERED")
	require.Empty(t, svc.appendID, "rejected input never reaches the API")
}

func TestAppendRejectsBothValueSources(t *testing.T) {
	svc := &fakeValuesService{}
	cmd := newLeafCmdWithFs(newAppendCmd, svc, "json", func(fs afero.Fs) {
		writeTestFile(t, fs, "/tmp/rows.json", `[[1]]`)
	})
	_, err := cmdtest.RunCmdErr(t, cmd, seedSpreadsheetID,
		"--range", "Sheet1!A1:D", "--values", `[[1]]`, "--values-file", "/tmp/rows.json")

	require.Contains(t, err.Error(), "one source only")
	require.Contains(t, err.Error(), "not both")
	require.Empty(t, svc.appendID)
}

func TestAppendValuesFileJSON(t *testing.T) {
	svc := &fakeValuesService{}
	cmd := newLeafCmdWithFs(newAppendCmd, svc, "json", func(fs afero.Fs) {
		writeTestFile(t, fs, "/tmp/rows.json", `[[1,"a"],[2,"b"]]`)
	})
	cmdtest.RunCmd(t, cmd, seedSpreadsheetID, "--range", "Sheet1!A1:D", "--values-file", "/tmp/rows.json")

	require.Equal(t, [][]any{{json.Number("1"), "a"}, {json.Number("2"), "b"}}, svc.appended)
}

func TestAppendValuesFileCSVCellsAreStrings(t *testing.T) {
	svc := &fakeValuesService{}
	cmd := newLeafCmdWithFs(newAppendCmd, svc, "json", func(fs afero.Fs) {
		writeTestFile(t, fs, "/tmp/rows.csv", "1,a\n2,b\n")
	})
	cmdtest.RunCmd(t, cmd, seedSpreadsheetID, "--range", "Sheet1!A1:D", "--values-file", "/tmp/rows.csv")

	require.Equal(t, [][]any{{"1", "a"}, {"2", "b"}}, svc.appended)
}

func TestAppendValuesFileTSV(t *testing.T) {
	svc := &fakeValuesService{}
	cmd := newLeafCmdWithFs(newAppendCmd, svc, "json", func(fs afero.Fs) {
		writeTestFile(t, fs, "/tmp/rows.tsv", "1\ta\n2\tb\n")
	})
	cmdtest.RunCmd(t, cmd, seedSpreadsheetID, "--range", "Sheet1!A1:D", "--values-file", "/tmp/rows.tsv")

	require.Equal(t, [][]any{{"1", "a"}, {"2", "b"}}, svc.appended)
}

func TestAppendValuesFileUnsupportedExtension(t *testing.T) {
	svc := &fakeValuesService{}
	cmd := newLeafCmdWithFs(newAppendCmd, svc, "json", func(fs afero.Fs) {
		writeTestFile(t, fs, "/tmp/rows.txt", "1,a\n")
	})
	_, err := cmdtest.RunCmdErr(t, cmd, seedSpreadsheetID, "--range", "Sheet1!A1:D", "--values-file", "/tmp/rows.txt")

	require.Contains(t, err.Error(), "unsupported values file extension")
}

func TestAppendMissingValues(t *testing.T) {
	svc := &fakeValuesService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newAppendCmd, svc, "json"), seedSpreadsheetID, "--range", "Sheet1!A1:D")

	require.Contains(t, err.Error(), "no values given")
	require.Empty(t, svc.appendID)
}

func TestAppendRequiresRange(t *testing.T) {
	svc := &fakeValuesService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newAppendCmd, svc, "json"), seedSpreadsheetID, "--values", `[[1]]`)

	require.Contains(t, err.Error(), `required flag(s) "range" not set`)
}

func TestAppendPropagatesAPIError(t *testing.T) {
	svc := &fakeValuesService{appendErr: errors.New("googleapi: Error 400")}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newAppendCmd, svc, "json"), seedSpreadsheetID,
		"--range", "Sheet1!A1:D", "--values", `[[1]]`)

	require.Contains(t, err.Error(), "googleapi: Error 400")
}

func TestAppendTableUpperCasesHeaders(t *testing.T) {
	svc := &fakeValuesService{}
	out := cmdtest.RunCmd(t, newLeafCmd(newAppendCmd, svc, "table"), seedSpreadsheetID,
		"--range", "Sheet1!A1:D", "--values", `[[1]]`)

	for _, header := range []string{"UPDATED_RANGE", "UPDATED_ROWS", "UPDATED_COLS"} {
		require.Contains(t, out, header)
	}
}
