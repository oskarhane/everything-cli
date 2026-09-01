package docs

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestDeleteRefusesWithoutForce(t *testing.T) {
	svc := cmdtest.NewDeleteRecorder()
	_, err := cmdtest.RunCmdErr(t, newFileLeafCmd(newDeleteCmd, svc, "json"), "doc_1")

	// The refusal wording is contractual: it names the document, the
	// --force remedy, and the recoverable alternative, and no service call
	// may have happened.
	require.ErrorContains(t, err, `refusing to permanently delete document "doc_1" without --force`)
	require.ErrorContains(t, err, `use "everything-cli drive file trash <id>" instead`)
	require.Empty(t, svc.DeletedIDs)
}

func TestDeleteWithForce(t *testing.T) {
	svc := cmdtest.NewDeleteRecorder()
	cmdtest.RunCmd(t, newFileLeafCmd(newDeleteCmd, svc, "json"), "doc_1", "--force")

	require.Equal(t, []string{"doc_1"}, svc.DeletedIDs)
}

func TestDeletePropagatesAPIError(t *testing.T) {
	svc := &cmdtest.DeleteRecorder{Err: errAPI}
	_, err := cmdtest.RunCmdErr(t, newFileLeafCmd(newDeleteCmd, svc, "json"), "doc_1", "--force")

	require.ErrorIs(t, err, errAPI)
}

func TestDeleteRequiresExactlyOneArg(t *testing.T) {
	svc := cmdtest.NewDeleteRecorder()
	_, err := cmdtest.RunCmdErr(t, newFileLeafCmd(newDeleteCmd, svc, "json"))

	require.Contains(t, err.Error(), "accepts 1 arg")
}
