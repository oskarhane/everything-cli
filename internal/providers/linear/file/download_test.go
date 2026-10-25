package file

import (
	"errors"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

// testFileURL is the file-storage URL every download fixture answers for.
const testFileURL = "https://uploads.linear.app/8c8a1b2c-0000-4000-8000-000000000001"

// TestDownloadStreamsToStdout: download streams the bytes to stdout
// byte-for-byte — control bytes included, no framing or stripping — and
// hands the leaf's URL through to the service untouched.
func TestDownloadStreamsToStdout(t *testing.T) {
	svc := &fakeService{content: []byte("a\x00b\x07c\xff")}
	out := cmdtest.RunCmd(t, newLeafCmd(newDownloadCmd, svc, "json"), testFileURL)

	require.Equal(t, []byte("a\x00b\x07c\xff"), []byte(out))
	require.Equal(t, testFileURL, svc.gotURL)
}

// TestDownloadOutWritesBytes: --out streams the bytes through the afero FS,
// creating parent directories, and stdout stays empty.
func TestDownloadOutWritesBytes(t *testing.T) {
	svc := &fakeService{content: []byte{0x00, 0x01, 0xFF, 0x25}}
	fs := afero.NewMemMapFs()
	cmd := newLeafCmdWithFs(newDownloadCmd, svc, "json", fs)
	out := cmdtest.RunCmd(t, cmd, testFileURL, "--out", "dl/nested/screenshot.png")

	require.Empty(t, out)
	require.Equal(t, []byte{0x00, 0x01, 0xFF, 0x25}, readAll(t, fs, "dl/nested/screenshot.png"))
	require.Equal(t, testFileURL, svc.gotURL)
}

// TestDownloadPropagatesServiceError: the service's error surfaces
// unchanged, like other linear calls.
func TestDownloadPropagatesServiceError(t *testing.T) {
	svc := &fakeService{err: errors.New("linear API returned 401 Unauthorized")}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newDownloadCmd, svc, "json"), testFileURL)

	require.Contains(t, err.Error(), "linear API returned 401 Unauthorized")
}

// TestDownloadOutRemovesFileOnFailure: a failed download with --out removes
// the destination — WriteToFile truncates or creates it before streaming —
// so an allowlist-refused URL, a non-200, or a mid-stream failure leaves
// no zero-byte or partial file behind.
func TestDownloadOutRemovesFileOnFailure(t *testing.T) {
	tests := []struct {
		name      string
		svc       *fakeService
		errSubstr string
	}{
		{
			name:      "allowlist-refused URL",
			svc:       &fakeService{err: errors.New("linear file download refused URL: only https://uploads.linear.app is allowed")},
			errSubstr: "refused URL",
		},
		{
			name:      "non-200 answer",
			svc:       &fakeService{err: errors.New("linear API returned 502 Bad Gateway: boom")},
			errSubstr: "linear API returned 502",
		},
		{
			name:      "mid-stream failure",
			svc:       &fakeService{content: []byte("partial bytes"), streamErr: errors.New("streaming linear file download: connection reset by peer")},
			errSubstr: "streaming linear file download",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := afero.NewMemMapFs()
			cmd := newLeafCmdWithFs(newDownloadCmd, tt.svc, "json", fs)
			_, err := cmdtest.RunCmdErr(t, cmd, testFileURL, "--out", "dl/screenshot.png")

			require.ErrorContains(t, err, tt.errSubstr)
			exists, ferr := afero.Exists(fs, "dl/screenshot.png")
			require.NoError(t, ferr)
			require.False(t, exists, "a failed download must remove --out; no zero-byte or partial file may be left")
		})
	}
}

// TestDownloadRequiresExactlyOneArg: the url positional is mandatory.
func TestDownloadRequiresExactlyOneArg(t *testing.T) {
	svc := &fakeService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newDownloadCmd, svc, "json"))

	require.Contains(t, err.Error(), "accepts 1 arg")
}
