package file

import (
	"errors"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	drive "google.golang.org/api/drive/v3"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestDownloadStreamsBlobToStdout(t *testing.T) {
	svc := &fakeService{
		files:       []*drive.File{seedBlobFile("application/pdf")},
		downloadedB: []byte("PDF BYTES"),
	}
	out := cmdtest.RunCmd(t, newLeafCmd(newDownloadCmd, svc, "json"), "file_1")

	// Binary content reaches stdout byte-for-byte, no framing or stripping.
	require.Equal(t, "PDF BYTES", string(out))
	require.Equal(t, "file_1", svc.downloaded)
	require.Empty(t, svc.exportedID, "blob must not be exported")
}

func TestDownloadNativeDocDefaultsToTextPlain(t *testing.T) {
	svc := &fakeService{
		files:       []*drive.File{seedBlobFile("application/vnd.google-apps.document")},
		exportBytes: []byte("doc text"),
	}
	out := cmdtest.RunCmd(t, newLeafCmd(newDownloadCmd, svc, "json"), "file_1")

	require.Equal(t, "file_1", svc.exportedID)
	require.Equal(t, "text/plain", svc.exportMime)
	require.Equal(t, "doc text", string(out))
}

func TestDownloadNativeSheetDefaultsToCSV(t *testing.T) {
	svc := &fakeService{
		files:       []*drive.File{seedBlobFile("application/vnd.google-apps.spreadsheet")},
		exportBytes: []byte("a,b\n1,2"),
	}
	out := cmdtest.RunCmd(t, newLeafCmd(newDownloadCmd, svc, "json"), "file_1")

	require.Equal(t, "text/csv", svc.exportMime)
	require.Equal(t, "a,b\n1,2", string(out))
}

func TestDownloadNativeSlideDefaultsToTextPlain(t *testing.T) {
	svc := &fakeService{
		files:       []*drive.File{seedBlobFile("application/vnd.google-apps.presentation")},
		exportBytes: []byte("slide text"),
	}
	cmdtest.RunCmd(t, newLeafCmd(newDownloadCmd, svc, "json"), "file_1")

	require.Equal(t, "text/plain", svc.exportMime)
}

func TestDownloadExportFlagWins(t *testing.T) {
	svc := &fakeService{
		files:       []*drive.File{seedBlobFile("application/vnd.google-apps.spreadsheet")},
		exportBytes: []byte("xlsx bytes"),
	}
	out := cmdtest.RunCmd(t, newLeafCmd(newDownloadCmd, svc, "json"), "file_1", "--export",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")

	require.Equal(t, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", svc.exportMime)
	require.Equal(t, "xlsx bytes", string(out))
}

func TestDownloadNativeWithoutDefaultRefuses(t *testing.T) {
	for _, mimeType := range []string{
		"application/vnd.google-apps.folder",
		"application/vnd.google-apps.drawing",
		"application/vnd.google-apps.form",
	} {
		t.Run(mimeType, func(t *testing.T) {
			svc := &fakeService{files: []*drive.File{seedBlobFile(mimeType)}}
			_, err := cmdtest.RunCmdErr(t, newLeafCmd(newDownloadCmd, svc, "json"), "file_1")

			require.Error(t, err)
			require.Contains(t, err.Error(), "no default text export")
			require.Contains(t, err.Error(), mimeType)
			// The refusal must teach the export matrix, including the
			// first-sheet-only caveat for spreadsheets.
			require.Contains(t, err.Error(), "first sheet only")
			require.Contains(t, err.Error(), "image/svg+xml")
		})
	}
}

func TestDownloadOutWritesBytes(t *testing.T) {
	svc := &fakeService{
		files:       []*drive.File{seedBlobFile("application/pdf")},
		downloadedB: []byte{0x00, 0x01, 0xFF, 0x25},
	}
	fs := afero.NewMemMapFs()
	cmd := newLeafCmdWithFs(newDownloadCmd, svc, "json", fs)
	cmdtest.RunCmd(t, cmd, "file_1", "--out", "dl/nested/report.pdf")

	require.Equal(t, "file_1", svc.downloaded)
	require.Equal(t, []byte{0x00, 0x01, 0xFF, 0x25}, readAll(t, fs, "dl/nested/report.pdf"))
}

func TestDownloadStdoutVerbatim(t *testing.T) {
	// Control bytes (e.g. NUL, BEL) must survive untouched: download output is
	// data, never redacted or control-stripped.
	svc := &fakeService{
		files:       []*drive.File{seedBlobFile("application/octet-stream")},
		downloadedB: []byte("a\x00b\x07c"),
	}
	out := cmdtest.RunCmd(t, newLeafCmd(newDownloadCmd, svc, "json"), "file_1")

	require.Equal(t, []byte("a\x00b\x07c"), []byte(out))
}

func TestDownloadPropagatesAPIError(t *testing.T) {
	svc := &fakeService{err: errors.New("googleapi: Error 403: access denied")}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newDownloadCmd, svc, "json"), "file_1")

	require.Contains(t, err.Error(), "googleapi: Error 403")
}

func TestDownloadRequiresExactlyOneArg(t *testing.T) {
	svc := &fakeService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newDownloadCmd, svc, "json"))

	require.Contains(t, err.Error(), "accepts 1 arg")
}
