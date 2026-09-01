package docs

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/google-cli/internal/subcommands/cmdtest"
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
