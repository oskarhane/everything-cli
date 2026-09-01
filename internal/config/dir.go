// Package config resolves everything-cli's config directory and persists
// accounts and settings on an injectable afero filesystem.
package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// EnvConfigDir overrides the config directory when set, so tests and
// sandboxes never touch the real ~/.config/everything-cli.
const EnvConfigDir = "EVERYTHING_CLI_CONFIG_DIR"

// LegacyEnvConfigDir is the pre-rename spelling of EnvConfigDir. It is
// still honored when EnvConfigDir is unset, with a deprecation warning on
// stderr, for one major version.
const LegacyEnvConfigDir = "GOOGLE_CLI_CONFIG_DIR"

// defaultDirName is the config directory name under ~/.config; legacyDirName
// is its pre-rename spelling, migrated by copy-over on first run.
const (
	defaultDirName = "everything-cli"
	legacyDirName  = "google-cli"
)

// stderr receives the legacy-env deprecation warning; tests swap it to
// capture output. Only env var names and dir paths are ever written here —
// never account or token content.
var stderr io.Writer = os.Stderr

// ResolveDir returns the everything-cli config directory.
//
// Precedence: an explicit root (tests, embedding callers), then
// $EVERYTHING_CLI_CONFIG_DIR, then $GOOGLE_CLI_CONFIG_DIR (deprecated —
// warns on stderr), then ~/.config/everything-cli. The directory is not
// created; the store creates it on first write.
func ResolveDir(root string) (string, error) {
	dir, _, err := resolveDir(root)
	return dir, err
}

// resolveDir implements ResolveDir and additionally reports whether the
// result is the built-in default (~/.config/everything-cli), so NewStore
// can trigger first-run copy-over migration only for default resolution —
// never for an explicit or env-provided dir.
func resolveDir(root string) (dir string, isDefault bool, err error) {
	if root != "" {
		return root, false, nil
	}
	if env := os.Getenv(EnvConfigDir); env != "" {
		return env, false, nil
	}
	if env := os.Getenv(LegacyEnvConfigDir); env != "" {
		// Best-effort: a failed warning write must not break resolution.
		_, _ = fmt.Fprintf(stderr, "warning: %s is deprecated; use %s instead\n", LegacyEnvConfigDir, EnvConfigDir)
		return env, false, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false, fmt.Errorf("resolving home directory for config dir: %w", err)
	}
	return filepath.Join(home, ".config", defaultDirName), true, nil
}
