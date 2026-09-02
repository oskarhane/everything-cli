package docs

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestGetStreamsTextToStdoutVerbatim(t *testing.T) {
	svc := &fakeDocService{docText: seedDocText()}
	out := cmdtest.RunCmd(t, newLeafCmd(newGetCmd, svc, "json"), "doc_1")

	// The export is content, not a report: bytes pass through raw, control
	// bytes included, with no format framing of any kind.
	require.Equal(t, seedDocText(), out)
}

func TestGetOutWritesFile(t *testing.T) {
	svc := &fakeDocService{docText: seedDocText()}
	fs := afero.NewMemMapFs()
	cmd := newLeafCmdWithFs(newGetCmd, svc, "json", fs)

	cmdtest.RunCmd(t, cmd, "doc_1", "--out", "out/notes.txt")

	// Bytes land on the memmap FS verbatim, parent dir created.
	require.Equal(t, []byte(seedDocText()), readAll(t, fs, "out/notes.txt"))
}

func TestGetOverwritesExistingOut(t *testing.T) {
	svc := &fakeDocService{docText: "fresh"}
	fs := afero.NewMemMapFs()
	seedTextFile(t, fs, "out/notes.txt", "stale content much longer than the new text")
	cmd := newLeafCmdWithFs(newGetCmd, svc, "json", fs)

	cmdtest.RunCmd(t, cmd, "doc_1", "--out", "out/notes.txt")

	require.Equal(t, []byte("fresh"), readAll(t, fs, "out/notes.txt"))
}

func TestGetPropagatesAPIError(t *testing.T) {
	svc := &fakeDocService{err: errAPI}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newGetCmd, svc, "json"), "doc_1")

	require.ErrorIs(t, err, errAPI)
}

func TestGetRequiresExactlyOneArg(t *testing.T) {
	svc := &fakeDocService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newGetCmd, svc, "json"))

	require.Contains(t, err.Error(), "accepts 1 arg")
}
