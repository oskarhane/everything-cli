package event

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestDeleteInstanceCancelsOnlyThatOccurrence(t *testing.T) {
	svc := &fakeEventService{}
	cmdtest.RunCmd(t, newLeafCmd(newDeleteCmd, svc, "json"), instanceEventID, "--force")

	require.Len(t, svc.deletes, 1)
	d := svc.deletes[0]
	require.Equal(t, instanceEventID, d.eventID, "deleting an instance id cancels one occurrence")
	require.Equal(t, "primary", d.calendarID)
	require.Equal(t, "all", d.sendUpdates)
}

func TestDeleteMasterDeletesTheSeries(t *testing.T) {
	svc := &fakeEventService{}
	cmdtest.RunCmd(t, newLeafCmd(newDeleteCmd, svc, "json"), masterEventID, "--force")

	require.Len(t, svc.deletes, 1)
	require.Equal(t, masterEventID, svc.deletes[0].eventID, "deleting a master id deletes the entire series")
}

func TestDeleteThisOnlyFalseWithInstanceDeletesSeries(t *testing.T) {
	svc := &fakeEventService{}
	cmdtest.RunCmd(t, newLeafCmd(newDeleteCmd, svc, "json"), instanceEventID, "--force", "--this-only=false")

	require.Len(t, svc.deletes, 1)
	require.Equal(t, masterEventID, svc.deletes[0].eventID, "--this-only=false resolves the instance id to its master")
}

func TestDeleteInstanceWithoutForceExplainsOccurrenceScope(t *testing.T) {
	svc := &fakeEventService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newDeleteCmd, svc, "json"), instanceEventID)

	require.Contains(t, err.Error(), "without --force")
	require.Contains(t, err.Error(), "cancels 1 occurrence")
	require.Empty(t, svc.deletes)
}

func TestDeleteMasterWithoutForceExplainsSeriesScope(t *testing.T) {
	svc := &fakeEventService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newDeleteCmd, svc, "json"), masterEventID)

	require.Contains(t, err.Error(), "without --force")
	require.Contains(t, err.Error(), "deletes the entire series")
	require.Empty(t, svc.deletes)
}

func TestDeletePropagatesAPIError(t *testing.T) {
	svc := &fakeEventService{deleteErr: errors.New("googleapi: Error 403")}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newDeleteCmd, svc, "json"), masterEventID, "--force")

	require.Contains(t, err.Error(), "googleapi: Error 403")
}

func TestDeleteRequiresExactlyOneArg(t *testing.T) {
	svc := &fakeEventService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newDeleteCmd, svc, "json"))

	require.Contains(t, err.Error(), "accepts 1 arg")
}
