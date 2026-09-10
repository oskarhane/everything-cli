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

func TestGetTabByIDReadsTabText(t *testing.T) {
	svc := &fakeDocService{docTabs: seedDocTabs(), docTabText: "tab body\n"}
	out := cmdtest.RunCmd(t, newLeafCmd(newGetCmd, svc, "json"), "doc_1", "--tab", "t.def456")

	// The tab render is raw content too: same stdout path as the doc-wide
	// export, bytes verbatim, and the tab key reaches the service as given.
	require.Equal(t, "tab body\n", out)
	require.Equal(t, "t.def456", svc.tabReadID)
}

func TestGetTabByTitleForwardsKeyToService(t *testing.T) {
	svc := &fakeDocService{docTabs: seedDocTabs(), docTabText: "tab body\n"}
	out := cmdtest.RunCmd(t, newLeafCmd(newGetCmd, svc, "json"), "doc_1", "--tab", "Changelog")

	// get forwards the --tab key as-is; the service resolves it (exact tab
	// ID first, then exact title).
	require.Equal(t, "tab body\n", out)
	require.Equal(t, "Changelog", svc.tabReadID)
}

func TestGetTabOutWritesFile(t *testing.T) {
	svc := &fakeDocService{docTabs: seedDocTabs(), docTabText: "tab body\n"}
	fs := afero.NewMemMapFs()
	cmd := newLeafCmdWithFs(newGetCmd, svc, "json", fs)

	cmdtest.RunCmd(t, cmd, "doc_1", "--tab", "t.def456", "--out", "out/tab.txt")

	// --tab shares the --out plumbing with the doc-wide export.
	require.Equal(t, []byte("tab body\n"), readAll(t, fs, "out/tab.txt"))
}

func TestGetUnknownTabPropagatesServiceError(t *testing.T) {
	svc := &fakeDocService{docTabs: seedDocTabs(), docText: "DOC-WIDE-EXPORT-MARKER\n", docTabText: "TAB-RENDER-MARKER"}
	out, err := cmdtest.RunCmdErr(t, newLeafCmd(newGetCmd, svc, "json"), "doc_1", "--tab", "nope")

	// The unknown key fails the tab read; the leaf must not fall back to
	// the doc-wide export — no document bytes may reach stdout (the usage
	// block cobra prints on error is harness noise, not content).
	require.ErrorContains(t, err, `no tab with ID or title "nope"`)
	require.NotContains(t, out, "DOC-WIDE-EXPORT-MARKER")
	require.NotContains(t, out, "TAB-RENDER-MARKER")
}
