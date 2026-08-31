package update

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeExe writes data at path with the executable bit, like a real binary.
func writeExe(t *testing.T, path, data string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(data), 0o755))
}

// tempBinaryDir creates a dir with a fake installed binary (optionally
// reached through an install symlink) and wires SelfPath to it.
func tempBinary(t *testing.T, symlink bool) (target, installPath string) {
	t.Helper()
	dir := t.TempDir()
	target = filepath.Join(dir, "google-cli")
	writeExe(t, target, "old")
	installPath = target
	if symlink {
		installPath = filepath.Join(dir, "bin-link")
		require.NoError(t, os.Symlink(target, installPath))
	}
	orig := SelfPath
	SelfPath = func() (string, error) { return installPath, nil }
	t.Cleanup(func() { SelfPath = orig })
	return target, installPath
}

func TestReplaceBinarySuccess(t *testing.T) {
	target, installPath := tempBinary(t, true)
	newBin := filepath.Join(t.TempDir(), "new")
	writeExe(t, newBin, "new-payload")

	require.NoError(t, ReplaceBinary(newBin))

	// The RESOLVED target was replaced; the symlink itself survives.
	got, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "new-payload", string(got))

	linkDest, err := os.Readlink(installPath)
	require.NoError(t, err)
	assert.Equal(t, target, linkDest, "install path must stay a symlink to the same target")

	assertBinMode(t, target)

	// No leftover temp files in the binary directory.
	entries, err := os.ReadDir(filepath.Dir(target))
	require.NoError(t, err)
	for _, e := range entries {
		assert.NotContains(t, e.Name(), ".update-", "leftover temp file %q", e.Name())
	}
}

func TestReplaceBinaryDirectPath(t *testing.T) {
	// No symlink: the binary file itself is the resolved target.
	target, _ := tempBinary(t, false)
	newBin := filepath.Join(t.TempDir(), "new")
	writeExe(t, newBin, "replaced")

	require.NoError(t, ReplaceBinary(newBin))
	got, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "replaced", string(got))
	assertBinMode(t, target)
}

func TestReplaceBinaryFailureKeepsOriginal(t *testing.T) {
	target, _ := tempBinary(t, false)
	newBin := filepath.Join(t.TempDir(), "missing") // does not exist

	err := ReplaceBinary(newBin)
	require.Error(t, err)

	got, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "old", string(got), "original binary must be untouched on failure")

	entries, err := os.ReadDir(filepath.Dir(target))
	require.NoError(t, err)
	for _, e := range entries {
		assert.NotContains(t, e.Name(), ".update-", "no leftover temp file after failure")
	}
}

func TestReplaceBinaryUnsupportedPlatform(t *testing.T) {
	orig := hostOS
	hostOS = "windows"
	t.Cleanup(func() { hostOS = orig })

	err := ReplaceBinary(filepath.Join(t.TempDir(), "new"))
	require.ErrorIs(t, err, ErrUnsupportedPlatform)
}

func assertBinMode(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, fs.FileMode(replacePerm), info.Mode().Perm(), "binary must be 0755")
}
