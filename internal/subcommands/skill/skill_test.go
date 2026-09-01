package skill

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/app"
)

func TestNewCmdRegistersSubtrees(t *testing.T) {
	cmd := NewCmd(&app.Config{Fs: afero.NewMemMapFs()})

	require.Equal(t, "skill", cmd.Name())
	var names []string
	for _, sub := range cmd.Commands() {
		names = append(names, sub.Name())
	}
	require.Equal(t, []string{"install", "print", "remove"}, names)
}
