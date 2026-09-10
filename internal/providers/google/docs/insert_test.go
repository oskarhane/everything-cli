package docs

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestInsertPassesIndexAndTextThrough(t *testing.T) {
	svc := &fakeDocService{}
	out := cmdtest.RunCmd(t, newLeafCmd(newInsertCmd, svc, "json"),
		"doc_1", "--index", "1", "--text", "Q4 plan")

	// The index reaches the API untouched; the text is sent verbatim, with no
	// newline added (unlike append).
	require.Equal(t, "doc_1", svc.insertID)
	require.Equal(t, "Q4 plan", svc.insertText)
	require.Equal(t, int64(1), svc.insertIndex)
	require.Equal(t, "Inserted text into document doc_1 at index 1\n", out)
}

func TestInsertReadsTextFileVerbatim(t *testing.T) {
	svc := &fakeDocService{}
	fs := afero.NewMemMapFs()
	seedTextFile(t, fs, "block.txt", "from file")
	cmd := newLeafCmdWithFs(newInsertCmd, svc, "json", fs)

	cmdtest.RunCmd(t, cmd, "doc_1", "--text-file", "block.txt", "--index", "120")

	// No newline is added to file-sourced inserts: the text is the caller's,
	// byte for byte.
	require.Equal(t, "from file", svc.insertText)
	require.Equal(t, int64(120), svc.insertIndex)
}

func TestInsertRequiresPositiveIndex(t *testing.T) {
	svc := &fakeDocService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newInsertCmd, svc, "json"), "doc_1", "--text", "x")

	// The required-flag wording is contractual: --index is mandatory and >0.
	require.ErrorContains(t, err, "--index is required")
	require.Empty(t, svc.insertID)
}

func TestInsertRejectsNonPositiveIndex(t *testing.T) {
	svc := &fakeDocService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newInsertCmd, svc, "json"),
		"doc_1", "--index", "0", "--text", "x")

	require.ErrorContains(t, err, "--index is required")
	require.Empty(t, svc.insertID)
}

func TestInsertRejectsBothTextAndTextFile(t *testing.T) {
	svc := &fakeDocService{}
	fs := afero.NewMemMapFs()
	seedTextFile(t, fs, "block.txt", "from file")
	cmd := newLeafCmdWithFs(newInsertCmd, svc, "json", fs)

	_, err := cmdtest.RunCmdErr(t, cmd, "doc_1", "--index", "2", "--text", "inline", "--text-file", "block.txt")

	require.Contains(t, err.Error(), "--text and --text-file are mutually exclusive")
	require.Empty(t, svc.insertID)
}

func TestInsertRequiresTextOrTextFile(t *testing.T) {
	svc := &fakeDocService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newInsertCmd, svc, "json"), "doc_1", "--index", "1")

	// The required-flags wording is contractual.
	require.ErrorContains(t, err, "--text or --text-file is required")
	require.Empty(t, svc.insertID)
}

func TestInsertPropagatesAPIError(t *testing.T) {
	svc := &fakeDocService{err: errAPI}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newInsertCmd, svc, "json"),
		"doc_1", "--index", "1", "--text", "x")

	require.ErrorIs(t, err, errAPI)
}

func TestInsertRequiresExactlyOneArg(t *testing.T) {
	svc := &fakeDocService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newInsertCmd, svc, "json"), "--index", "1", "--text", "x")

	require.Contains(t, err.Error(), "accepts 1 arg")
}

func TestInsertTabByIDReachesService(t *testing.T) {
	svc := &fakeDocService{docTabs: seedDocTabs()}
	cmdtest.RunCmd(t, newLeafCmd(newInsertCmd, svc, "json"),
		"doc_1", "--index", "1", "--text", "x", "--tab", "t.def456")

	require.Equal(t, "t.def456", svc.insertTabID)
	require.Equal(t, int64(1), svc.insertIndex)
}

func TestInsertTabByTitleSendsResolvedID(t *testing.T) {
	svc := &fakeDocService{docTabs: seedDocTabs()}
	cmdtest.RunCmd(t, newLeafCmd(newInsertCmd, svc, "json"),
		"doc_1", "--index", "1", "--text", "x", "--tab", "Changelog")

	// InsertDocText makes no read of its own: the leaf resolves the title
	// via ListDocTabs and sends the resolved tab ID.
	require.Equal(t, "t.def456", svc.insertTabID)
}

func TestInsertUnknownTabWritesNothing(t *testing.T) {
	svc := &fakeDocService{docTabs: seedDocTabs()}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newInsertCmd, svc, "json"),
		"doc_1", "--index", "1", "--text", "x", "--tab", "nope")

	require.ErrorContains(t, err, `no tab with ID or title "nope"`)
	require.Empty(t, svc.insertID) // zero write calls
}

func TestInsertAmbiguousTabTitleErrors(t *testing.T) {
	svc := &fakeDocService{docTabs: append(seedDocTabs(),
		service.DocTab{TabID: "t.ghi789", Title: "Changelog", Index: 2, ParentTabID: "t.abc123"})}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newInsertCmd, svc, "json"),
		"doc_1", "--index", "1", "--text", "x", "--tab", "Changelog")

	// The error names the matching tabs so the caller can pick an ID.
	require.ErrorContains(t, err, `tab title "Changelog" is ambiguous`)
	require.ErrorContains(t, err, "t.def456, t.ghi789")
	require.Empty(t, svc.insertID) // zero write calls
}
