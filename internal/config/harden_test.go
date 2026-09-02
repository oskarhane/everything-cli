package config

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSaveLeavesEnvRootPermissionsAlone pins the ownership gate: a
// pre-existing root the user pointed at via $EVERYTHING_CLI_CONFIG_DIR is
// NOT chmodded on write — its permissions are the user's choice, and
// tightening them could break whatever else shares the dir. The store's own
// subdirs (accounts/, accounts/google/) are still created private.
// (Contrast: explicit and default roots ARE tightened — pinned by
// TestStoreSaveTightensWiderDirs in store_write_test.go.)
func TestSaveLeavesEnvRootPermissionsAlone(t *testing.T) {
	fsys := afero.NewMemMapFs()
	require.NoError(t, fsys.MkdirAll("/env/root", 0o755))
	t.Setenv(EnvConfigDir, "/env/root")

	store, err := NewStore(fsys, "")
	require.NoError(t, err)
	require.True(t, store.envRoot)
	require.NoError(t, store.Save(&Account{
		Name:  "work",
		Email: "work@example.com",
		Token: testToken("access-1"),
	}))

	info, err := fsys.Stat("/env/root")
	require.NoError(t, err)
	assert.Equal(t, fs.FileMode(0o755), info.Mode().Perm(),
		"env-pointed pre-existing root must keep the user's permissions")

	for _, dir := range []string{store.accountsDir(), store.providerDir(ProviderGoogle)} {
		info, err := fsys.Stat(dir)
		require.NoError(t, err)
		assert.Equal(t, fs.FileMode(0o700), info.Mode().Perm(),
			"store-owned subdir %s must still be private", dir)
	}
}

// TestSaveLeavesLegacyEnvRootPermissionsAlone: same gate via the deprecated
// $GOOGLE_CLI_CONFIG_DIR spelling.
func TestSaveLeavesLegacyEnvRootPermissionsAlone(t *testing.T) {
	fsys := afero.NewMemMapFs()
	require.NoError(t, fsys.MkdirAll("/env/root", 0o755))
	t.Setenv(EnvConfigDir, "")
	t.Setenv(LegacyEnvConfigDir, "/env/root")
	captureStderr(t) // swallow the deprecation warning

	store, err := NewStore(fsys, "")
	require.NoError(t, err)
	require.NoError(t, store.Save(&Account{
		Name:  "work",
		Email: "work@example.com",
		Token: testToken("access-1"),
	}))

	info, err := fsys.Stat("/env/root")
	require.NoError(t, err)
	assert.Equal(t, fs.FileMode(0o755), info.Mode().Perm())
}

// TestHardenDirReplacesSymlinkedDir: hardenDir must Lstat, never follow —
// a symlinked accounts dir would otherwise redirect the chmod (and later
// writes) to the link target. The symlink is unlinked and recreated as a
// real private dir; the target outside the config dir is untouched.
// Symlinks need a real filesystem, so this uses afero.NewOsFs() against a
// throwaway t.TempDir() — the sanctioned exception (store_write_test.go).
func TestHardenDirReplacesSymlinkedDir(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(root, "outside")
	require.NoError(t, os.MkdirAll(outside, 0o755))

	realFs := afero.NewOsFs()
	store, err := NewStore(realFs, filepath.Join(root, "config"))
	require.NoError(t, err)
	require.NoError(t, realFs.MkdirAll(store.root, 0o700))
	require.NoError(t, os.Symlink(outside, store.accountsDir()))

	require.NoError(t, store.Save(&Account{
		Name:  "work",
		Email: "work@example.com",
		Token: testToken("access-1"),
	}))

	info, err := os.Stat(outside)
	require.NoError(t, err)
	assert.Equal(t, fs.FileMode(0o755), info.Mode().Perm(),
		"hardenDir must not follow the symlink and chmod the outside dir")

	info, err = os.Lstat(store.accountsDir())
	require.NoError(t, err)
	assert.Equal(t, fs.FileMode(0), info.Mode()&os.ModeSymlink,
		"the symlink must be unlinked, not followed")
	assert.True(t, info.IsDir(), "accounts dir must be recreated as a real dir")
	assert.Equal(t, fs.FileMode(0o700), info.Mode().Perm())

	// The account landed inside the config dir, not at the symlink target.
	got, err := store.Get("work")
	require.NoError(t, err)
	assert.Equal(t, "work@example.com", got.Email)
	entries, err := os.ReadDir(outside)
	require.NoError(t, err)
	assert.Empty(t, entries, "no writes may escape through the symlink")
}
