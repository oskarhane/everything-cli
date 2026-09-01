package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests exercise on-disk permission, atomicity and symlink semantics,
// so they use afero.NewOsFs() against a throwaway t.TempDir() — the
// sanctioned exception to the in-memory FS rule (CLAUDE.md). No test here
// ever points at the real ~/.config/google-cli.

func newOsStore(t *testing.T) (*Store, afero.Fs) {
	t.Helper()
	realFs := afero.NewOsFs()
	store, err := NewStore(realFs, t.TempDir())
	require.NoError(t, err)
	return store, realFs
}

func osFilePerm(t *testing.T, fs afero.Fs, path string) fs.FileMode {
	t.Helper()
	info, err := fs.Stat(path)
	require.NoError(t, err)
	return info.Mode().Perm()
}

// TestStoreSaveTightensPreexisting0600: S5 — afero.WriteFile perm only
// applies at creation, so an account file that already exists with wider
// perms used to keep them through every refresh. Save must tighten it.
func TestStoreSaveTightensPreexisting0600(t *testing.T) {
	store, realFs := newOsStore(t)

	path := store.AccountPath("work")
	require.NoError(t, realFs.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte(`{"name":"work"}`), 0o644))

	require.NoError(t, store.Save(&Account{
		Name:  "work",
		Email: "work@example.com",
		Token: testToken("access-1"),
	}))

	assert.Equal(t, fs.FileMode(0o600), osFilePerm(t, realFs, path),
		"existing 0644 account file must be tightened to 0600 on Save")
}

// TestStoreSaveTightensWiderDirs: the config root and accounts dir are
// tightened to 0700 when they pre-exist with wider perms. The root is a
// nested dir inside t.TempDir() (which is already 0700) so it can start wide.
func TestStoreSaveTightensWiderDirs(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "cfg")
	realFs := afero.NewOsFs()
	store, err := NewStore(realFs, nested)
	require.NoError(t, err)
	require.NoError(t, realFs.MkdirAll(nested, 0o755))
	require.NoError(t, realFs.Mkdir(store.accountsDir(), 0o755))

	require.NoError(t, store.Save(&Account{
		Name:  "work",
		Email: "work@example.com",
		Token: testToken("access-1"),
	}))

	assert.Equal(t, fs.FileMode(0o700), osFilePerm(t, realFs, store.root),
		"pre-existing 0755 config dir must be tightened to 0700")
	assert.Equal(t, fs.FileMode(0o700), osFilePerm(t, realFs, store.accountsDir()),
		"pre-existing 0755 accounts dir must be tightened to 0700")
}

// TestStoreSetDefaultAccountTightensPreexisting0600: S5 for config.json.
func TestStoreSetDefaultAccountTightensPreexisting0600(t *testing.T) {
	store, realFs := newOsStore(t)

	require.NoError(t, store.Save(&Account{Name: "work", Email: "work@example.com", Token: testToken("a")}))
	// Pre-existing settings file with wide perms, plus a wide config root.
	require.NoError(t, realFs.Chmod(store.root, 0o755))
	settingsPath := store.settingsPath()
	require.NoError(t, os.WriteFile(settingsPath, []byte(`{}`), 0o644))
	require.NoError(t, realFs.Chmod(store.root, 0o755))

	require.NoError(t, store.SetDefaultAccount("work"))

	assert.Equal(t, fs.FileMode(0o600), osFilePerm(t, realFs, settingsPath),
		"existing 0644 settings file must be tightened to 0600")
	assert.Equal(t, fs.FileMode(0o700), osFilePerm(t, realFs, store.root),
		"pre-existing 0755 config dir must be tightened to 0700")
}

// TestStoreSaveLeavesNoTmpResidue observes the temp-then-rename write
// directly: after a successful save the accounts dir holds exactly the
// account file — no temp file survives.
func TestStoreSaveLeavesNoTmpResidue(t *testing.T) {
	store, _ := newOsStore(t)

	for i := 0; i < 3; i++ {
		require.NoError(t, store.Save(&Account{
			Name:  "work",
			Email: "work@example.com",
			Token: testToken("access-" + string(rune('0'+i))),
		}))
	}

	entries, err := os.ReadDir(store.providerDir(ProviderGoogle))
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	assert.Equal(t, []string{"work.json"}, names,
		"temp files must be renamed away on success, never left behind")

	data, err := os.ReadFile(store.AccountPath("work"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "access-2", "final save wins")
}

// failingWriteFs wraps an afero.Fs and injects write failures on regular
// files, simulating a crash mid-write.
type failingWriteFs struct {
	afero.Fs
	failWrites bool
}

func (f *failingWriteFs) OpenFile(name string, flag int, perm fs.FileMode) (afero.File, error) {
	fh, err := f.Fs.OpenFile(name, flag, perm)
	if err != nil {
		return nil, err
	}
	if f.failWrites {
		return &failingFile{File: fh}, nil
	}
	return fh, nil
}

type failingFile struct {
	afero.File
}

func (f *failingFile) Write(p []byte) (int, error) {
	return 0, errors.New("injected write failure")
}

// TestStoreSaveFailureKeepsOldContent: S7 — an injected mid-write failure
// must leave the previous account JSON fully intact at the target path and
// clean up the temp file. No partial state is ever observable.
func TestStoreSaveFailureKeepsOldContent(t *testing.T) {
	dir := t.TempDir()
	realFs := afero.NewOsFs()
	store, err := NewStore(realFs, dir)
	require.NoError(t, err)

	good := &Account{Name: "work", Email: "work@example.com", Token: testToken("access-1")}
	require.NoError(t, store.Save(good))
	before, err := os.ReadFile(store.AccountPath("work"))
	require.NoError(t, err)

	broken := &failingWriteFs{Fs: realFs, failWrites: true}
	store.fs = broken
	err = store.Save(&Account{Name: "work", Email: "work@example.com", Token: testToken("access-2")})
	require.Error(t, err, "injected write failure must surface")
	assert.Contains(t, err.Error(), "injected write failure")

	after, err := os.ReadFile(store.AccountPath("work"))
	require.NoError(t, err, "failed save must not remove the old account file")
	assert.Equal(t, string(before), string(after),
		"failed save must leave the old content byte-identical — no partial JSON")

	entries, err := os.ReadDir(store.accountsDir())
	require.NoError(t, err)
	assert.Len(t, entries, 1, "temp file must be removed on failure")

	// Recovery: a good save still lands after the failure.
	store.fs = realFs
	require.NoError(t, store.Save(&Account{Name: "work", Email: "work@example.com", Token: testToken("access-2")}))
	got, err := store.Get("work")
	require.NoError(t, err)
	assert.Equal(t, "access-2", got.Token.AccessToken)
}

// TestStoreSaveReplacesSymlink: S8 — OsFs write paths follow symlinks, so a
// symlinked accounts/google/<name>.json used to redirect the token write outside
// the config dir. The temp+rename write replaces the symlink instead: the
// outside file is untouched and the path becomes a regular file.
func TestStoreSaveReplacesSymlink(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(root, "outside")
	require.NoError(t, os.MkdirAll(outside, 0o700))
	outsidePath := filepath.Join(outside, "evil.json")
	// Must be parseable account JSON: Save's findByEmail reads every
	// accounts/google/*.json, following the symlink on read before writing.
	outsideJSON := `{"name":"work","email":"attacker@example.com"}` + "\n"
	require.NoError(t, os.WriteFile(outsidePath, []byte(outsideJSON), 0o600))

	realFs := afero.NewOsFs()
	store, err := NewStore(realFs, filepath.Join(root, "config"))
	require.NoError(t, err)
	require.NoError(t, realFs.MkdirAll(filepath.Dir(store.AccountPath("work")), 0o700))
	require.NoError(t, os.Symlink(outsidePath, store.AccountPath("work")))

	require.NoError(t, store.Save(&Account{
		Name:  "work",
		Email: "work@example.com",
		Token: testToken("access-1"),
	}))

	outsideAfter, err := os.ReadFile(outsidePath)
	require.NoError(t, err)
	assert.Equal(t, outsideJSON, string(outsideAfter),
		"symlink target outside the config dir must not be clobbered")

	info, err := os.Lstat(store.AccountPath("work"))
	require.NoError(t, err)
	assert.Equal(t, fs.FileMode(0), info.Mode()&os.ModeSymlink,
		"path must no longer be a symlink — rename replaces the link")
	assert.True(t, info.Mode().IsRegular(), "path must now be a regular file")
	assert.Equal(t, fs.FileMode(0o600), info.Mode().Perm())

	data, err := os.ReadFile(store.AccountPath("work"))
	require.NoError(t, err)
	assert.True(t, strings.Contains(string(data), "work@example.com"),
		"token write must land in the config dir, not at the symlink target")
}
