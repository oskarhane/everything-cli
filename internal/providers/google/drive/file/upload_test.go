package file

import (
	"errors"
	"mime"
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

const pptxMime = "application/vnd.openxmlformats-officedocument.presentationml.presentation"

// pptxFs seeds a .pptx in a memmap FS. The type is registered explicitly so
// the media content-type assertion does not depend on the host's mime.types.
func pptxFs(t *testing.T) afero.Fs {
	t.Helper()
	require.NoError(t, mime.AddExtensionType(".pptx", pptxMime))
	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/local/deck.pptx", []byte("PPTX bytes"), 0o644))
	return fs
}

// TestUploadConvertToSlideConversion asserts both multipart parts: the
// metadata MimeType is the Google Slides target, while the media content-type
// stays the real .pptx source type so Drive can perform the conversion.
func TestUploadConvertToSlideConversion(t *testing.T) {
	svc := &fakeService{}
	cmdtest.RunCmd(t, newLeafCmdWithFs(newUploadCmd, svc, "json", pptxFs(t)),
		"/local/deck.pptx", "--convert-to", "slide")

	require.Equal(t, "application/vnd.google-apps.presentation", svc.uploaded.MimeType)
	require.Equal(t, pptxMime, svc.uploadMime)
	require.Equal(t, []byte("PPTX bytes"), svc.uploadBytes)
}

func TestUploadConvertToFullMimePassthrough(t *testing.T) {
	svc := &fakeService{}
	cmdtest.RunCmd(t, newLeafCmdWithFs(newUploadCmd, svc, "json", pptxFs(t)),
		"/local/deck.pptx", "--convert-to", "application/vnd.google-apps.document")

	require.Equal(t, "application/vnd.google-apps.document", svc.uploaded.MimeType)
	require.Equal(t, pptxMime, svc.uploadMime)
}

func TestUploadConvertToEveryShorthand(t *testing.T) {
	tests := []struct {
		convertTo string
		want      string
	}{
		{"folder", "application/vnd.google-apps.folder"},
		{"doc", "application/vnd.google-apps.document"},
		{"sheet", "application/vnd.google-apps.spreadsheet"},
		{"slide", "application/vnd.google-apps.presentation"},
	}
	for _, tt := range tests {
		t.Run(tt.convertTo, func(t *testing.T) {
			svc := &fakeService{}
			cmdtest.RunCmd(t, newLeafCmdWithFs(newUploadCmd, svc, "json", newUploadFs(t)),
				"/local/report.pdf", "--convert-to", tt.convertTo)

			require.Equal(t, tt.want, svc.uploaded.MimeType)
		})
	}
}

func TestUploadConvertToRejectsNonGoogleTarget(t *testing.T) {
	svc := &fakeService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmdWithFs(newUploadCmd, svc, "json", newUploadFs(t)),
		"/local/report.pdf", "--convert-to", "application/pdf")

	require.Contains(t, err.Error(), `unsupported --convert-to "application/pdf"`)
	require.Contains(t, err.Error(), "application/vnd.google-apps.")
}

func TestUploadConvertToRejectsUnknownShorthand(t *testing.T) {
	svc := &fakeService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmdWithFs(newUploadCmd, svc, "json", newUploadFs(t)),
		"/local/report.pdf", "--convert-to", "slides")

	require.Contains(t, err.Error(), `unsupported --convert-to "slides"`)
}

func TestUploadConvertToConflictsWithMimeType(t *testing.T) {
	svc := &fakeService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmdWithFs(newUploadCmd, svc, "json", newUploadFs(t)),
		"/local/report.pdf", "--convert-to", "doc", "--mime-type", "application/pdf")

	require.Contains(t, err.Error(), "--convert-to")
	require.Contains(t, err.Error(), "--mime-type")
	require.Contains(t, err.Error(), "mutually exclusive")
}

// TestUploadWithoutConvertToLeavesMetadataMimeEmpty pins the default path: no
// --convert-to must not populate the metadata MimeType (preserving existing
// behavior byte for byte).
func TestUploadWithoutConvertToLeavesMetadataMimeEmpty(t *testing.T) {
	svc := &fakeService{}
	cmdtest.RunCmd(t, newLeafCmdWithFs(newUploadCmd, svc, "json", newUploadFs(t)), "/local/report.pdf")

	require.Empty(t, svc.uploaded.MimeType)
	require.Equal(t, "application/pdf", svc.uploadMime)
}
