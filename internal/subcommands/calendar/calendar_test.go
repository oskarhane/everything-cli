package calendar

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/google-cli/internal/app"
)

func TestNewCmdRegistersSubtrees(t *testing.T) {
	cmd := NewCmd(&app.Config{Fs: afero.NewMemMapFs()})

	require.Equal(t, "calendar", cmd.Name())
	var names []string
	for _, sub := range cmd.Commands() {
		names = append(names, sub.Name())
	}
	// cobra sorts subcommands alphabetically: the acl and event subgroups
	// plus the calendar CRUD leaves hanging directly off the parent.
	require.Equal(t, []string{"acl", "create", "delete", "event", "get", "list", "update"}, names)
}
