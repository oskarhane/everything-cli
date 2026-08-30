package skill

import (
	"os"
	"path/filepath"
	"strings"
)

// getenv is a seam so tests can drive environment lookups hermetically via
// t.Setenv without touching the real environment of the host process.
var getenv = os.Getenv

const tokenXDGConfigHome = "$XDG_CONFIG_HOME"

// expandPath resolves a path containing `~` or `$XDG_CONFIG_HOME`.
//   - `~` alone -> $HOME
//   - `~/foo`   -> $HOME/foo
//   - `$XDG_CONFIG_HOME` -> that env var, falling back to $HOME/.config
//     when unset or empty
//   - other paths are returned unchanged
//
// Returns ok=false only when the environment cannot supply the base the
// path needs (no $HOME).
func expandPath(path string) (string, bool) {
	home := getenv("HOME")

	if path == "~" {
		if home == "" {
			return "", false
		}
		return home, true
	}
	if rest, ok := strings.CutPrefix(path, "~/"); ok {
		if home == "" {
			return "", false
		}
		return filepath.Join(home, rest), true
	}
	if strings.Contains(path, tokenXDGConfigHome) {
		base, ok := xdgConfigDir(home)
		if !ok {
			return "", false
		}
		// Catalog entries keep forward slashes (portable convention) but
		// the substituted base may be OS-native; FromSlash normalises.
		return filepath.FromSlash(strings.ReplaceAll(path, tokenXDGConfigHome, base)), true
	}
	return path, true
}

// xdgConfigDir resolves $XDG_CONFIG_HOME, falling back to $HOME/.config
// when unset or empty.
func xdgConfigDir(home string) (string, bool) {
	if xdg := getenv("XDG_CONFIG_HOME"); xdg != "" {
		return xdg, true
	}
	if home == "" {
		return "", false
	}
	return filepath.Join(home, ".config"), true
}
