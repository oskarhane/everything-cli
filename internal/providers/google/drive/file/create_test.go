package file

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestCreateJSON(t *testing.T) {
	svc := &fakeService{}
	out := cmdtest.RunCmd(t, newLeafCmd(newCreateCmd, svc, "json"), "Reports", "--parent", "1AbC", "--description", "Q3")

	row, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok, "expected a JSON object, got: %s", out)
	keys := cmdtest.JSONKeys(t, row)
	cmdtest.RequireSnakeCase(t, keys)
	require.Equal(t, "file_new", row["id"])
}

func TestCreateSendsSpec(t *testing.T) {
	svc := &fakeService{}
	cmdtest.RunCmd(t, newLeafCmd(newCreateCmd, svc, "json"), "Reports",
		"--type", "doc", "--parent", "1AbC", "--description", "Weekly notes")

	require.Equal(t, "Reports", svc.created.Name)
	require.Equal(t, "application/vnd.google-apps.document", svc.created.MimeType)
	require.Equal(t, []string{"1AbC"}, svc.created.Parents)
	require.Equal(t, "Weekly notes", svc.created.Description)
}

func TestCreateDefaultTypeIsFolder(t *testing.T) {
	svc := &fakeService{}
	cmdtest.RunCmd(t, newLeafCmd(newCreateCmd, svc, "json"), "Archive")

	require.Equal(t, "application/vnd.google-apps.folder", svc.created.MimeType)
}

func TestCreateEachType(t *testing.T) {
	tests := []struct {
		typeFlag string
		want     string
	}{
		{"folder", "application/vnd.google-apps.folder"},
		{"doc", "application/vnd.google-apps.document"},
		{"sheet", "application/vnd.google-apps.spreadsheet"},
		{"slide", "application/vnd.google-apps.presentation"},
	}
	for _, tt := range tests {
		t.Run(tt.typeFlag, func(t *testing.T) {
			svc := &fakeService{}
			cmdtest.RunCmd(t, newLeafCmd(newCreateCmd, svc, "json"), "x", "--type", tt.typeFlag)

			require.Equal(t, tt.want, svc.created.MimeType)
		})
	}
}

func TestCreateRawMimeType(t *testing.T) {
	svc := &fakeService{}
	cmdtest.RunCmd(t, newLeafCmd(newCreateCmd, svc, "json"), "photo", "--mime-type", "image/png")

	require.Equal(t, "image/png", svc.created.MimeType)
}

func TestCreateTypeAndMimeTypeMutuallyExclusive(t *testing.T) {
	svc := &fakeService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newCreateCmd, svc, "json"), "x",
		"--type", "doc", "--mime-type", "text/plain")

	require.Contains(t, err.Error(), "--type")
	require.Contains(t, err.Error(), "--mime-type")
	require.Contains(t, err.Error(), "mutually exclusive")
}

func TestCreateRawMimeTypeWinsOverDefaultType(t *testing.T) {
	// --mime-type alone must suppress the --type default (folder) rather than
	// fail the XOR check against it.
	svc := &fakeService{}
	cmdtest.RunCmd(t, newLeafCmd(newCreateCmd, svc, "json"), "notes.md", "--mime-type", "text/markdown")

	require.Equal(t, "text/markdown", svc.created.MimeType)
}

func TestCreateUnknownType(t *testing.T) {
	svc := &fakeService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newCreateCmd, svc, "json"), "x", "--type", "slides")

	require.Contains(t, err.Error(), `unsupported --type "slides"`)
	require.Contains(t, err.Error(), "folder, doc, sheet, or slide")
}

func TestCreatePropagatesAPIError(t *testing.T) {
	svc := &fakeService{err: errors.New("googleapi: Error 500")}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newCreateCmd, svc, "json"), "x")

	require.Contains(t, err.Error(), "googleapi: Error 500")
}

func TestCreateRequiresExactlyOneArg(t *testing.T) {
	svc := &fakeService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newCreateCmd, svc, "json"), "a", "b")

	require.Contains(t, err.Error(), "accepts 1 arg")
}
