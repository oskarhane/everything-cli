package calendar

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/google-cli/internal/config"
)

// newTestStore returns a hermetic store on an in-memory FS: tests must never
// read or write the real ~/.config/google-cli.
func newTestStore(t *testing.T) *config.Store {
	t.Helper()
	store, err := config.NewStore(afero.NewMemMapFs(), "/config")
	require.NoError(t, err)
	return store
}

// seedAccount persists an account so resolution has something to find.
func seedAccount(t *testing.T, store *config.Store, name string) {
	t.Helper()
	require.NoError(t, store.Save(&config.Account{Name: name, Email: name + "@example.com"}))
}
