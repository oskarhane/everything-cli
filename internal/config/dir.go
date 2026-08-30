// Package config resolves google-cli's config directory and persists
// accounts and settings on an injectable afero filesystem.
package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// EnvConfigDir overrides the config directory when set, so tests and
// sandboxes never touch the real ~/.config/google-cli.
const EnvConfigDir = "GOOGLE_CLI_CONFIG_DIR"

// ResolveDir returns the google-cli config directory.
//
// Precedence: an explicit root (tests, embedding callers), then
// $GOOGLE_CLI_CONFIG_DIR, then ~/.config/google-cli. The directory is not
// created; the store creates it on first write.
func ResolveDir(root string) (string, error) {
	if root != "" {
		return root, nil
	}
	if env := os.Getenv(EnvConfigDir); env != "" {
		return env, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home directory for config dir: %w", err)
	}
	return filepath.Join(home, ".config", "google-cli"), nil
}
