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

// dirSource records how the config dir was resolved, so the store can tell
// an env-pointed root (user-shared, never chmodded) from one it owns.
type dirSource int

const (
	sourceExplicit dirSource = iota
	sourceEnv
	sourceDefault
)

// ResolveDir returns the everything-cli config directory.
//
// Precedence: an explicit root (tests, embedding callers), then
// $EVERYTHING_CLI_CONFIG_DIR, then $GOOGLE_CLI_CONFIG_DIR (deprecated —
// warns on stderr), then ~/.config/everything-cli. A relative env value is
// absolutized against the current working directory, with a stderr warning,
// so the token store's location never shifts with the caller's CWD. The
// directory is not created; the store creates it on first write.
func ResolveDir(root string) (string, error) {
	dir, _, err := resolveDir(root)
	return dir, err
}

// resolveDir implements ResolveDir and additionally reports how the result
// was resolved, so NewStore can trigger first-run copy-over migration only
// for default resolution — never for an explicit or env-provided dir — and
// the store can skip chmodding an env-pointed root it does not own.
func resolveDir(root string) (dir string, source dirSource, err error) {
	if root != "" {
		return root, sourceExplicit, nil
	}
	if env := os.Getenv(EnvConfigDir); env != "" {
		dir, err := absolutizeEnvDir(EnvConfigDir, env)
		return dir, sourceEnv, err
	}
	if env := os.Getenv(LegacyEnvConfigDir); env != "" {
		// Best-effort: a failed warning write must not break resolution.
		_, _ = fmt.Fprintf(stderr, "warning: %s is deprecated; use %s instead\n", LegacyEnvConfigDir, EnvConfigDir)
		dir, err := absolutizeEnvDir(LegacyEnvConfigDir, env)
		return dir, sourceEnv, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", sourceDefault, fmt.Errorf("resolving home directory for config dir: %w", err)
	}
	return filepath.Join(home, ".config", defaultDirName), sourceDefault, nil
}

// absolutizeEnvDir anchors a relative env-provided config dir to the current
// working directory, warning on stderr: silently honoring it would make the
// token store's location depend on the caller's CWD, so the same relative
// value could point at different directories from different invocations.
func absolutizeEnvDir(name, value string) (string, error) {
	if filepath.IsAbs(value) {
		return value, nil
	}
	abs, err := filepath.Abs(value)
	if err != nil {
		return "", fmt.Errorf("absolutizing relative %s %q: %w", name, value, err)
	}
	// Best-effort: a failed warning write must not break resolution.
	_, _ = fmt.Fprintf(stderr, "warning: %s %q is relative; using %q\n", name, value, abs)
	return abs, nil
}
