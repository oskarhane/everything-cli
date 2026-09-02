package calendarlist

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestDeleteRefusesWithoutForce(t *testing.T) {
	svc := &fakeService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newDeleteCmd, svc, "json"), "cal_99")

	require.Contains(t, err.Error(), `refusing to delete calendar "cal_99" without --force`)
	require.False(t, svc.deleteCalled, "delete must not reach the API without --force")
}

func TestDeleteWithForce(t *testing.T) {
	svc := &fakeService{}
	out := cmdtest.RunCmd(t, newLeafCmd(newDeleteCmd, svc, "json"), "cal_99", "--force")

	require.True(t, svc.deleteCalled)
	require.Equal(t, "cal_99", svc.deletedID)
	require.Empty(t, out, "successful delete is silent")
}

func TestDeletePropagatesAPIError(t *testing.T) {
	svc := &fakeService{deleteErr: errors.New("googleapi: Error 400")}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newDeleteCmd, svc, "json"), "cal_99", "--force")

	require.Contains(t, err.Error(), "googleapi: Error 400")
}
