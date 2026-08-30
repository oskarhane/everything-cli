package skill

import (
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// newTestFS returns an in-memory FS with HOME pinned to home and
// XDG_CONFIG_HOME cleared, so expandPath resolves into a hermetic tree
// regardless of the host environment.
func newTestFS(t *testing.T, home string) afero.Fs {
	t.Helper()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	return afero.NewMemMapFs()
}

// mkdir seeds a directory on the test FS (agents are detected by DetectDir
// presence).
func mkdir(t *testing.T, fs afero.Fs, dir string) {
	t.Helper()
	require.NoError(t, fs.MkdirAll(dir, 0o755))
}

// homePath is a helper building a $HOME-anchored path in tests.
func homePath(home, rest string) string {
	return filepath.Join(home, rest)
}
