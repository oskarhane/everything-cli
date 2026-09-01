package file

import (
	"errors"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

// newUploadFs returns a memmap FS with one local file seeded for upload tests.
func newUploadFs(t *testing.T) afero.Fs {
	t.Helper()
	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/local/report.pdf", []byte("PDF bytes"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/local/noext", []byte("raw bytes"), 0o644))
	return fs
}

func TestUploadSendsLocalBytes(t *testing.T) {
	svc := &fakeService{}
	cmd := newLeafCmdWithFs(newUploadCmd, svc, "json", newUploadFs(t))
	out := cmdtest.RunCmd(t, cmd, "/local/report.pdf", "--parent", "1AbC")

	// The local bytes must ride the upload unmodified.
	require.Equal(t, []byte("PDF bytes"), svc.uploadBytes)
	require.Equal(t, "report.pdf", svc.uploaded.Name)
	require.Equal(t, []string{"1AbC"}, svc.uploaded.Parents)

	row := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.Equal(t, "file_new", row["id"])
}

func TestUploadDefaultsNameToBase(t *testing.T) {
	svc := &fakeService{}
	cmdtest.RunCmd(t, newLeafCmdWithFs(newUploadCmd, svc, "json", newUploadFs(t)), "/local/report.pdf")

	require.Equal(t, "report.pdf", svc.uploaded.Name)
}

func TestUploadNameFlagOverrides(t *testing.T) {
	svc := &fakeService{}
	cmdtest.RunCmd(t, newLeafCmdWithFs(newUploadCmd, svc, "json", newUploadFs(t)),
		"/local/report.pdf", "--name", "Q3 report.pdf")

	require.Equal(t, "Q3 report.pdf", svc.uploaded.Name)
}

func TestUploadMimeFromExtension(t *testing.T) {
	svc := &fakeService{}
	cmdtest.RunCmd(t, newLeafCmdWithFs(newUploadCmd, svc, "json", newUploadFs(t)), "/local/report.pdf")

	require.Equal(t, "application/pdf", svc.uploadMime)
}

func TestUploadMimeFlagWins(t *testing.T) {
	svc := &fakeService{}
	cmdtest.RunCmd(t, newLeafCmdWithFs(newUploadCmd, svc, "json", newUploadFs(t)),
		"/local/report.pdf", "--mime-type", "application/custom")

	require.Equal(t, "application/custom", svc.uploadMime)
}

func TestUploadUnknownExtensionFallsBackToOctetStream(t *testing.T) {
	svc := &fakeService{}
	cmdtest.RunCmd(t, newLeafCmdWithFs(newUploadCmd, svc, "json", newUploadFs(t)), "/local/noext")

	require.Equal(t, "application/octet-stream", svc.uploadMime)
}

func TestUploadMissingLocalFile(t *testing.T) {
	svc := &fakeService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmdWithFs(newUploadCmd, svc, "json", afero.NewMemMapFs()), "/no/such/file")

	require.Contains(t, err.Error(), "opening local file /no/such/file")
}

func TestUploadPropagatesAPIError(t *testing.T) {
	svc := &fakeService{err: errors.New("googleapi: Error 500")}
	_, err := cmdtest.RunCmdErr(t, newLeafCmdWithFs(newUploadCmd, svc, "json", newUploadFs(t)), "/local/report.pdf")

	require.Contains(t, err.Error(), "googleapi: Error 500")
}

func TestUploadRequiresExactlyOneArg(t *testing.T) {
	svc := &fakeService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmdWithFs(newUploadCmd, svc, "json", newUploadFs(t)))

	require.Contains(t, err.Error(), "accepts 1 arg")
}
