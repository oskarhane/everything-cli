package relation

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestDeletePassesRelationID(t *testing.T) {
	svc := &fakeService{}
	out := cmdtest.RunCmd(t, deleteCmd(svc, "json"), "rel_1")

	require.Equal(t, "rel_1", svc.deletedID, "the positional relation id passes through")
	require.Empty(t, out, "a successful delete prints nothing")
}

func TestDeleteRequiresRelationID(t *testing.T) {
	svc := &fakeService{}
	_, err := cmdtest.RunCmdErr(t, deleteCmd(svc, "json"))

	require.Error(t, err, "cobra rejects a missing relation id before RunE")
	require.False(t, svc.dialed, "the service is never dialed without a relation id")
}

func TestDeleteError(t *testing.T) {
	svc := &fakeService{err: errors.New("linear API reported issueRelationDelete success: false")}
	_, err := cmdtest.RunCmdErr(t, deleteCmd(svc, "json"), "rel_999")

	require.ErrorContains(t, err, "issueRelationDelete")
}
