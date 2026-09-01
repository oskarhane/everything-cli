package docs

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestAppendSendsTextWithTrailingNewline(t *testing.T) {
	svc := &fakeDocService{}
	cmdtest.RunCmd(t, newLeafCmd(newAppendCmd, svc, "json"), "doc_1", "--text", "Reviewed by Oskar")

	require.Equal(t, "doc_1", svc.appendedID)
	// Successive appends must each start on their own line, so a missing
	// trailing newline is added before the API call.
	require.Equal(t, "Reviewed by Oskar\n", svc.appendedText)
}

func TestAppendKeepsExistingTrailingNewline(t *testing.T) {
	svc := &fakeDocService{}
	cmdtest.RunCmd(t, newLeafCmd(newAppendCmd, svc, "json"), "doc_1", "--text", "already ends\n")

	require.Equal(t, "already ends\n", svc.appendedText)
}

func TestAppendReadsTextFile(t *testing.T) {
	svc := &fakeDocService{}
	fs := afero.NewMemMapFs()
	seedTextFile(t, fs, "notes.txt", "from file")
	cmd := newLeafCmdWithFs(newAppendCmd, svc, "json", fs)

	cmdtest.RunCmd(t, cmd, "doc_1", "--text-file", "notes.txt")

	require.Equal(t, "from file\n", svc.appendedText)
}

func TestAppendRejectsBothTextAndTextFile(t *testing.T) {
	svc := &fakeDocService{}
	fs := afero.NewMemMapFs()
	seedTextFile(t, fs, "notes.txt", "from file")
	cmd := newLeafCmdWithFs(newAppendCmd, svc, "json", fs)

	_, err := cmdtest.RunCmdErr(t, cmd, "doc_1", "--text", "inline", "--text-file", "notes.txt")

	require.Contains(t, err.Error(), "--text and --text-file are mutually exclusive")
	require.Empty(t, svc.appendedID)
}

func TestAppendRequiresTextOrTextFile(t *testing.T) {
	svc := &fakeDocService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newAppendCmd, svc, "json"), "doc_1")

	// The required-flags wording is contractual.
	require.ErrorContains(t, err, "--text or --text-file is required")
	require.Empty(t, svc.appendedID)
}

func TestAppendMissingTextFile(t *testing.T) {
	svc := &fakeDocService{}
	fs := afero.NewMemMapFs()
	cmd := newLeafCmdWithFs(newAppendCmd, svc, "json", fs)

	_, err := cmdtest.RunCmdErr(t, cmd, "doc_1", "--text-file", "nope.txt")

	require.ErrorContains(t, err, "reading --text-file nope.txt")
}

func TestAppendPropagatesAPIError(t *testing.T) {
	svc := &fakeDocService{err: errAPI}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newAppendCmd, svc, "json"), "doc_1", "--text", "hi")

	require.ErrorIs(t, err, errAPI)
}

func TestAppendRequiresExactlyOneArg(t *testing.T) {
	svc := &fakeDocService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newAppendCmd, svc, "json"), "--text", "hi")

	require.Contains(t, err.Error(), "accepts 1 arg")
}
