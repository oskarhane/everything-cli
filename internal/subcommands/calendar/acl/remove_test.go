package acl

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRemoveCallsAPI(t *testing.T) {
	svc := &fakeService{}
	out := runCmd(t, newLeafCmd(newRemoveCmd, svc, "json"),
		"primary", "--rule-id", "user:colleague@example.com")

	require.True(t, svc.deleteCalled)
	require.Equal(t, "primary", svc.deletedCalID)
	require.Equal(t, "user:colleague@example.com", svc.deletedRuleID)
	require.Empty(t, out, "successful remove is silent")
}

func TestRemoveRequiresRuleID(t *testing.T) {
	svc := &fakeService{}
	_, err := runCmdErr(t, newLeafCmd(newRemoveCmd, svc, "json"), "primary")

	require.Contains(t, err.Error(), "--rule-id is required")
	require.False(t, svc.deleteCalled, "missing rule id must not reach the API")
}

func TestRemovePropagatesAPIError(t *testing.T) {
	svc := &fakeService{deleteErr: errors.New("googleapi: Error 404")}
	_, err := runCmdErr(t, newLeafCmd(newRemoveCmd, svc, "json"),
		"primary", "--rule-id", "user:colleague@example.com")

	require.Contains(t, err.Error(), "googleapi: Error 404")
}
