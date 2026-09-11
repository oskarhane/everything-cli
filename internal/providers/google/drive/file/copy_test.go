package file

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	drive "google.golang.org/api/drive/v3"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestCopySendsNameAndPrintsJSON(t *testing.T) {
	svc := &fakeService{}
	out := cmdtest.RunCmd(t, newLeafCmd(newCopyCmd, svc, "json"), "src-id", "--name", "Deck")

	require.Equal(t, "src-id", svc.copiedFrom)
	require.Equal(t, "Deck", svc.copied.Name)
	row, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok, "expected a JSON object, got: %s", out)
	cmdtest.RequireSnakeCase(t, cmdtest.JSONKeys(t, row))
	require.Equal(t, "file_copy", row["id"])
	require.Equal(t, "Deck", row["name"])
}

func TestCopySendsParent(t *testing.T) {
	svc := &fakeService{}
	cmdtest.RunCmd(t, newLeafCmd(newCopyCmd, svc, "json"), "src-id", "--parent", "folder_9")

	require.Equal(t, []string{"folder_9"}, svc.copied.Parents)
	require.Empty(t, svc.copied.Name, "no --name means Drive keeps the source's name")
}

func TestCopyNativeFileCarriesNoMimeTypeOverride(t *testing.T) {
	// Drive keeps the source's type on a copy: a Google-native source must
	// reach Files.Copy with no mimeType in the metadata, or the copy would be
	// coerced to a different type.
	svc := &fakeService{
		files: []*drive.File{{Id: "src-id", Name: "Notes", MimeType: "application/vnd.google-apps.document"}},
	}
	out := cmdtest.RunCmd(t, newLeafCmd(newCopyCmd, svc, "json"), "src-id", "--name", "Notes copy")

	require.Empty(t, svc.copied.MimeType, "files.copy metadata must never set mimeType")
	row := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.Equal(t, "file_copy", row["id"])
}

func TestCopyPrintsIDInEveryFormat(t *testing.T) {
	for _, format := range []string{"json", "table", "toon"} {
		t.Run(format, func(t *testing.T) {
			svc := &fakeService{}
			out := cmdtest.RunCmd(t, newLeafCmd(newCopyCmd, svc, format), "src-id")

			// The copy's id is the payload every agent branches on; it must
			// survive each output format.
			require.Contains(t, out, "file_copy", "format %s must carry the copy's id", format)
		})
	}
}

func TestCopyPropagatesAPIError(t *testing.T) {
	svc := &fakeService{err: errors.New("googleapi: Error 404: file not found")}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newCopyCmd, svc, "json"), "src-id")

	require.Contains(t, err.Error(), "googleapi: Error 404")
}

func TestCopyRequiresExactlyOneArg(t *testing.T) {
	svc := &fakeService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newCopyCmd, svc, "json"))

	require.Contains(t, err.Error(), "accepts 1 arg")
}
