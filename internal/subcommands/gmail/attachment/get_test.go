package attachment

import (
	"errors"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/google-cli/internal/subcommands/cmdtest"
)

// seedPNG is a small realistic binary payload with a non-UTF-8 byte, so the
// tests catch any string-coercion of the raw bytes.
var seedPNG = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 'p', 'i', 'x', 'e', 'l', 's'}

func TestGetWritesBytesToStdout(t *testing.T) {
	svc := &fakeService{content: seedPNG}
	cmd := newGetCmd(cmdtest.NewTestConfig("json"), fakeNewSvc(svc))
	out := cmdtest.RunCmd(t, cmd, "ANG1xQ8q", "--message-id", "msg_1")

	require.Equal(t, seedPNG, []byte(out), "without --out the leaf must write the exact decoded bytes")
	require.Equal(t, "msg_1", svc.messageID)
	require.Equal(t, "ANG1xQ8q", svc.attachmentID)
}

func TestGetWritesFileViaAfero(t *testing.T) {
	cfg := cmdtest.NewTestConfig("json")
	svc := &fakeService{content: seedPNG}

	cmdtest.RunCmd(t, newGetCmd(cfg, fakeNewSvc(svc)), "ANG1xQ8q", "--message-id", "msg_1", "--out", "downloads/pics/logo.png")

	content, err := afero.ReadFile(cfg.Fs, "downloads/pics/logo.png")
	require.NoError(t, err, "--out must create parent dirs and write through the afero FS")
	require.Equal(t, seedPNG, content)
}

func TestGetOutCreatesMissingParentDirs(t *testing.T) {
	cfg := cmdtest.NewTestConfig("json")
	svc := &fakeService{content: seedPNG}

	cmdtest.RunCmd(t, newGetCmd(cfg, fakeNewSvc(svc)), "ANG1xQ8q", "--message-id", "msg_1", "--out", "a/b/c.bin")

	content, err := afero.ReadFile(cfg.Fs, "a/b/c.bin")
	require.NoError(t, err, "--out must create its parent dirs before writing")
	require.Equal(t, seedPNG, content)
}

func TestGetRequiresMessageID(t *testing.T) {
	svc := &fakeService{content: seedPNG}
	_, err := cmdtest.RunCmdErr(t, newGetCmd(cmdtest.NewTestConfig("json"), fakeNewSvc(svc)), "ANG1xQ8q")

	require.Contains(t, err.Error(), "--message-id is required")
	require.Empty(t, svc.messageID, "a missing --message-id must not reach the API")
}

func TestGetPropagatesAPIError(t *testing.T) {
	svc := &fakeService{err: errors.New("googleapi: Error 404: attachment not found")}
	_, err := cmdtest.RunCmdErr(t, newGetCmd(cmdtest.NewTestConfig("json"), fakeNewSvc(svc)),
		"ANG1xQ8q", "--message-id", "msg_1")

	require.Contains(t, err.Error(), "googleapi: Error 404")
}

func TestGetRequiresExactlyOneArg(t *testing.T) {
	svc := &fakeService{content: seedPNG}
	_, err := cmdtest.RunCmdErr(t, newGetCmd(cmdtest.NewTestConfig("json"), fakeNewSvc(svc)), "--message-id", "msg_1")

	require.Contains(t, err.Error(), "accepts 1 arg")
}
