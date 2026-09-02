package label

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestDeleteRefusesWithoutForce(t *testing.T) {
	svc := &fakeService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newDeleteCmd, svc, "json"), "Label_7")

	require.Contains(t, err.Error(), `refusing to delete label "Label_7" without --force`)
	require.False(t, svc.deleteCalled, "delete must not reach the API without --force")
}

func TestDeleteWithForce(t *testing.T) {
	svc := &fakeService{}
	out := cmdtest.RunCmd(t, newLeafCmd(newDeleteCmd, svc, "json"), "Label_7", "--force")

	require.True(t, svc.deleteCalled)
	require.Equal(t, "Label_7", svc.deletedID)
	require.Empty(t, out, "successful delete is silent")
}

func TestDeletePropagatesAPIError(t *testing.T) {
	svc := &fakeService{deleteErr: errors.New("googleapi: Error 400")}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newDeleteCmd, svc, "json"), "Label_7", "--force")

	require.Contains(t, err.Error(), "googleapi: Error 400")
}
