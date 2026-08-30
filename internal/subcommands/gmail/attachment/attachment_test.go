package attachment

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewCmdRegistersLeaves(t *testing.T) {
	cmd := NewCmd(newTestConfig("json"), fakeNewSvc(&fakeService{}))

	require.Equal(t, "attachment", cmd.Name())
	var names []string
	for _, sub := range cmd.Commands() {
		names = append(names, sub.Name())
	}
	require.Equal(t, []string{"get"}, names)
}
