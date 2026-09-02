package docs

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestReplacePrintsAPICount(t *testing.T) {
	svc := &fakeDocService{replaceCount: 3}
	out := cmdtest.RunCmd(t, newLeafCmd(newReplaceCmd, svc, "json"),
		"doc_1", "--find", "Falcon", "--replace-with", "Falcon 2")

	require.Equal(t, "Replaced 3 occurrence(s)\n", out)
}

func TestReplacePassesFlagsThrough(t *testing.T) {
	svc := &fakeDocService{replaceCount: 1}
	cmdtest.RunCmd(t, newLeafCmd(newReplaceCmd, svc, "json"),
		"doc_1", "--find", "falcon", "--replace-with", "hawk", "--match-case")

	require.Equal(t, "doc_1", svc.replaceID)
	require.Equal(t, "falcon", svc.replaceFind)
	require.Equal(t, "hawk", svc.replaceWith)
	require.True(t, svc.replaceCase)
}

func TestReplaceDefaultsToCaseInsensitive(t *testing.T) {
	svc := &fakeDocService{replaceCount: 2}
	cmdtest.RunCmd(t, newLeafCmd(newReplaceCmd, svc, "json"),
		"doc_1", "--find", "Falcon", "--replace-with", "Falcon 2")

	require.False(t, svc.replaceCase)
}

func TestReplaceEmptyReplacementIsAllowed(t *testing.T) {
	svc := &fakeDocService{replaceCount: 1}
	out := cmdtest.RunCmd(t, newLeafCmd(newReplaceCmd, svc, "json"),
		"doc_1", "--find", "TODO")

	// An empty --replace-with deletes the matches, which is legitimate.
	require.Equal(t, "", svc.replaceWith)
	require.Equal(t, "Replaced 1 occurrence(s)\n", out)
}

func TestReplaceRequiresFind(t *testing.T) {
	svc := &fakeDocService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newReplaceCmd, svc, "json"), "doc_1")

	// The refusal wording is contractual: --find is mandatory and non-empty.
	require.ErrorContains(t, err, "--find is required")
	require.Empty(t, svc.replaceID)
}

func TestReplacePropagatesAPIError(t *testing.T) {
	svc := &fakeDocService{err: errAPI}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newReplaceCmd, svc, "json"),
		"doc_1", "--find", "x")

	require.ErrorIs(t, err, errAPI)
}

func TestReplaceRequiresExactlyOneArg(t *testing.T) {
	svc := &fakeDocService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newReplaceCmd, svc, "json"), "--find", "x")

	require.Contains(t, err.Error(), "accepts 1 arg")
}
