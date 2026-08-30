package draft

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewCmdRegistersLeaves(t *testing.T) {
	cmd := NewCmd(newTestConfig("json"), fakeNewSvc(&fakeService{}))

	require.Equal(t, "draft", cmd.Name())
	var names []string
	for _, sub := range cmd.Commands() {
		names = append(names, sub.Name())
	}
	require.ElementsMatch(t, []string{"list", "get", "create", "send", "delete"}, names)
}
