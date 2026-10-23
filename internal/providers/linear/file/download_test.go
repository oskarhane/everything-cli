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

// TestDownloadRequiresExactlyOneArg: the url positional is mandatory.
func TestDownloadRequiresExactlyOneArg(t *testing.T) {
	svc := &fakeService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newDownloadCmd, svc, "json"))

	require.Contains(t, err.Error(), "accepts 1 arg")
}
