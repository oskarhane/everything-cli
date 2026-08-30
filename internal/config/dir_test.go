package config

import (
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveDir(t *testing.T) {
	home := t.TempDir()

	t.Run("explicit root wins over env", func(t *testing.T) {
		t.Setenv(EnvConfigDir, "/env/override")

		got, err := ResolveDir("/explicit/root")
		require.NoError(t, err)
		assert.Equal(t, "/explicit/root", got)
	})

	t.Run("env override", func(t *testing.T) {
		t.Setenv(EnvConfigDir, "/env/override")

		got, err := ResolveDir("")
		require.NoError(t, err)
		assert.Equal(t, "/env/override", got)
	})

	t.Run("empty env falls back to home", func(t *testing.T) {
		t.Setenv("HOME", home)
		t.Setenv(EnvConfigDir, "")

		got, err := ResolveDir("")
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(home, ".config", "google-cli"), got)
	})
}

func TestNewStoreUsesResolvedDir(t *testing.T) {
	store, err := NewStore(afero.NewMemMapFs(), "/custom/root")
	require.NoError(t, err)
	assert.Equal(t, "/custom/root", store.Dir())
	assert.Equal(t, "/custom/root/credentials.json", store.CredentialsPath())
	assert.Equal(t, "/custom/root/accounts/work.json", store.AccountPath("work"))
}
