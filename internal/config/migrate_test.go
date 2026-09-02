package config

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedLegacyDir writes a legacy google-cli fixture tree under home: one
// nested account file and a settings file.
func seedLegacyDir(t *testing.T, fsys afero.Fs, home string) (legacyRoot string) {
	t.Helper()
	legacyRoot = filepath.Join(home, ".config", "google-cli")
	require.NoError(t, fsys.MkdirAll(filepath.Join(legacyRoot, "accounts", "google"), 0o700))
	require.NoError(t, afero.WriteFile(fsys,
		filepath.Join(legacyRoot, "accounts", "google", "work.json"),
		[]byte(`{"name":"work"}`+"\n"), 0o600))
	require.NoError(t, afero.WriteFile(fsys,
		filepath.Join(legacyRoot, "config.json"),
		[]byte(`{"default_accounts":{"google":"work"}}`+"\n"), 0o600))
	return legacyRoot
}

func newRootFor(home string) string {
	return filepath.Join(home, ".config", "everything-cli")
}

func TestCopyLegacyDir(t *testing.T) {
	tests := []struct {
		name string
		// setup seeds the in-memory FS; returns the explicit/env dir to
		// pass to NewStore, or "" for default resolution.
		setup        func(t *testing.T, fsys afero.Fs, home string) (dirArg string)
		wantCopied   bool
		wantLegacyOK bool // legacy fixture (if seeded) must survive intact
	}{
		{
			name: "copies legacy tree when new dir absent",
			setup: func(t *testing.T, fsys afero.Fs, home string) string {
				seedLegacyDir(t, fsys, home)
				return ""
			},
			wantCopied:   true,
			wantLegacyOK: true,
		},
		{
			name: "no copy when new dir already exists",
			setup: func(t *testing.T, fsys afero.Fs, home string) string {
				seedLegacyDir(t, fsys, home)
				require.NoError(t, fsys.MkdirAll(newRootFor(home), 0o700))
				return ""
			},
			wantCopied:   false,
			wantLegacyOK: true,
		},
		{
			name: "no copy when legacy dir absent",
			setup: func(t *testing.T, fsys afero.Fs, home string) string {
				return ""
			},
			wantCopied: false,
		},
		{
			name: "no copy when resolution came from an explicit dir",
			setup: func(t *testing.T, fsys afero.Fs, home string) string {
				seedLegacyDir(t, fsys, home)
				return "/explicit/root"
			},
			wantCopied:   false,
			wantLegacyOK: true,
		},
		{
			name: "no copy when resolution came from the new env var",
			setup: func(t *testing.T, fsys afero.Fs, home string) string {
				seedLegacyDir(t, fsys, home)
				t.Setenv(EnvConfigDir, "/env/root")
				return ""
			},
			wantCopied:   false,
			wantLegacyOK: true,
		},
		{
			name: "no copy when resolution came from the legacy env var",
			setup: func(t *testing.T, fsys afero.Fs, home string) string {
				seedLegacyDir(t, fsys, home)
				t.Setenv(LegacyEnvConfigDir, "/env/root")
				captureStderr(t) // swallow the deprecation warning
				return ""
			},
			wantCopied:   false,
			wantLegacyOK: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir() // only used as a path prefix — never touched for real
			t.Setenv("HOME", home)
			fsys := afero.NewMemMapFs()
			dirArg := tc.setup(t, fsys, home)

			store, err := NewStore(fsys, dirArg)
			require.NoError(t, err)

			newRoot := newRootFor(home)
			if !tc.wantCopied {
				_, err := fsys.Stat(filepath.Join(newRoot, "accounts", "google", "work.json"))
				assert.ErrorIs(t, err, fs.ErrNotExist)
			} else {
				assert.Equal(t, newRoot, store.Dir())

				// Account file content copied.
				data, err := afero.ReadFile(fsys, filepath.Join(newRoot, "accounts", "google", "work.json"))
				require.NoError(t, err)
				assert.JSONEq(t, `{"name":"work"}`, string(data))

				// Settings file copied.
				data, err = afero.ReadFile(fsys, filepath.Join(newRoot, "config.json"))
				require.NoError(t, err)
				assert.JSONEq(t, `{"default_accounts":{"google":"work"}}`, string(data))

				// Private permission discipline: dirs 0700, files 0600.
				for _, dir := range []string{
					newRoot,
					filepath.Join(newRoot, "accounts"),
					filepath.Join(newRoot, "accounts", "google"),
				} {
					info, err := fsys.Stat(dir)
					require.NoError(t, err)
					assert.Equal(t, fs.FileMode(0o700), info.Mode().Perm(), "dir %s", dir)
				}
				info, err := fsys.Stat(filepath.Join(newRoot, "accounts", "google", "work.json"))
				require.NoError(t, err)
				assert.Equal(t, fs.FileMode(0o600), info.Mode().Perm())
			}

			if tc.wantLegacyOK {
				// The legacy tree is left intact — copy, never move.
				data, err := afero.ReadFile(fsys,
					filepath.Join(home, ".config", "google-cli", "accounts", "google", "work.json"))
				require.NoError(t, err)
				assert.JSONEq(t, `{"name":"work"}`, string(data))
			}
		})
	}
}

// crashFs simulates a crash mid-migration: the second migrated file's
// rename fails, leaving the first file complete at its target, the second
// present only as a temp file, and the marker in place.
type crashFs struct {
	afero.Fs
	renames int
}

func (f *crashFs) Rename(oldname, newname string) error {
	if strings.HasSuffix(oldname, ".tmp") {
		f.renames++
		if f.renames > 1 {
			return errors.New("injected crash")
		}
	}
	return f.Fs.Rename(oldname, newname)
}

// TestCopyLegacyDirResumesAfterCrash: a crash mid-copy must leave a state
// the next run completes — marker present, copied files intact, the crashed
// file never partially at its target, and no temp residue after resume.
func TestCopyLegacyDirResumesAfterCrash(t *testing.T) {
	home := t.TempDir() // only used as a path prefix — never touched for real
	t.Setenv("HOME", home)
	fsys := afero.NewMemMapFs()
	seedLegacyDir(t, fsys, home)
	newRoot := newRootFor(home)

	// First run crashes after the first file's rename lands.
	_, err := NewStore(&crashFs{Fs: fsys}, "")
	require.Error(t, err, "the injected crash must surface")
	assert.Contains(t, err.Error(), "injected crash")

	// Resumable state: marker present, the completed file is at its
	// target, the crashed file is absent there (never partial)...
	_, err = fsys.Stat(filepath.Join(newRoot, migrationMarker))
	require.NoError(t, err, "crashed migration must leave its marker behind")
	data, err := afero.ReadFile(fsys, filepath.Join(newRoot, "accounts", "google", "work.json"))
	require.NoError(t, err, "a completed copy survives the crash")
	assert.JSONEq(t, `{"name":"work"}`, string(data))
	_, err = fsys.Stat(filepath.Join(newRoot, "config.json"))
	assert.ErrorIs(t, err, fs.ErrNotExist,
		"temp+rename means a crashed copy never leaves partial content at the target")

	// The next run resumes and completes the migration.
	store, err := NewStore(fsys, "")
	require.NoError(t, err)
	assert.Equal(t, newRoot, store.Dir())
	data, err = afero.ReadFile(fsys, filepath.Join(newRoot, "config.json"))
	require.NoError(t, err, "resume must copy the file the crash skipped")
	assert.JSONEq(t, `{"default_accounts":{"google":"work"}}`, string(data))

	// No residue: marker cleared, crashed temp file redone and renamed away.
	_, err = fsys.Stat(filepath.Join(newRoot, migrationMarker))
	assert.ErrorIs(t, err, fs.ErrNotExist, "completed migration clears its marker")
	_, err = fsys.Stat(filepath.Join(newRoot, "config.json.tmp"))
	assert.ErrorIs(t, err, fs.ErrNotExist, "resume redoes the temp copy and renames it away")

	// The legacy tree is still intact — copy, never move.
	data, err = afero.ReadFile(fsys, filepath.Join(home, ".config", "google-cli", "config.json"))
	require.NoError(t, err)
	assert.JSONEq(t, `{"default_accounts":{"google":"work"}}`, string(data))

	// A third run is a no-op: the new dir now exists without a marker, so
	// it is treated as the user's own and left alone.
	_, err = NewStore(fsys, "")
	require.NoError(t, err)
	_, err = fsys.Stat(filepath.Join(newRoot, migrationMarker))
	assert.ErrorIs(t, err, fs.ErrNotExist)
}

// TestCopyLegacyDirHardensPermissions verifies migrated entries get private
// permissions even when the legacy fixture is world-readable.
func TestCopyLegacyDirHardensPermissions(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	fsys := afero.NewMemMapFs()
	legacyRoot := seedLegacyDir(t, fsys, home)
	require.NoError(t, fsys.Chmod(legacyRoot, 0o755))
	require.NoError(t, fsys.Chmod(filepath.Join(legacyRoot, "config.json"), 0o644))

	_, err := NewStore(fsys, "")
	require.NoError(t, err)

	info, err := fsys.Stat(newRootFor(home))
	require.NoError(t, err)
	assert.Equal(t, fs.FileMode(0o700), info.Mode().Perm())
	info, err = fsys.Stat(filepath.Join(newRootFor(home), "config.json"))
	require.NoError(t, err)
	assert.Equal(t, fs.FileMode(0o600), info.Mode().Perm())
}
