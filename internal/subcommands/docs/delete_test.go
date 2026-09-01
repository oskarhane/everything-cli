package docs

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/google-cli/internal/subcommands/cmdtest"
)

func TestDeleteRefusesWithoutForce(t *testing.T) {
	svc := &fakeFileService{}
	_, err := cmdtest.RunCmdErr(t, newFileLeafCmd(newDeleteCmd, svc, "json"), "doc_1")

	// The refusal wording is contractual: it names the document and the
	// --force remedy, and no service call may have happened.
	require.ErrorContains(t, err, `refusing to permanently delete document "doc_1" without --force`)
	require.False(t, svc.deleted)
}

func TestDeleteWithForce(t *testing.T) {
	svc := &fakeFileService{}
	cmdtest.RunCmd(t, newFileLeafCmd(newDeleteCmd, svc, "json"), "doc_1", "--force")

	require.True(t, svc.deleted)
	require.Equal(t, "doc_1", svc.deletedID)
}

func TestDeletePropagatesAPIError(t *testing.T) {
	svc := &fakeFileService{err: errAPI}
	_, err := cmdtest.RunCmdErr(t, newFileLeafCmd(newDeleteCmd, svc, "json"), "doc_1", "--force")

	require.ErrorIs(t, err, errAPI)
}

func TestDeleteRequiresExactlyOneArg(t *testing.T) {
	svc := &fakeFileService{}
	_, err := cmdtest.RunCmdErr(t, newFileLeafCmd(newDeleteCmd, svc, "json"))

	require.Contains(t, err.Error(), "accepts 1 arg")
}
