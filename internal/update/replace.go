package update

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

// ErrUnsupportedPlatform is returned when self-replacement is not supported
// on the current operating system.
var ErrUnsupportedPlatform = errors.New("self-replace not supported on this platform")

// SelfPath resolves the running executable. It is a package seam so tests
// can point ReplaceBinary at a fake executable.
var SelfPath = func() (string, error) { return os.Executable() }

// hostOS is a var so the windows gate is testable on any platform.
var hostOS = runtime.GOOS

// resolvedSelfPath is the symlink-resolved path of the running binary — the
// file ReplaceBinary targets and the executable the skill reinstall execs.
func resolvedSelfPath() (string, error) {
	self, err := SelfPath()
	if err != nil {
		return "", fmt.Errorf("resolving current executable: %w", err)
	}
	target, err := filepath.EvalSymlinks(self)
	if err != nil {
		return "", fmt.Errorf("resolving binary path %s: %w", self, err)
	}
	return target, nil
}

const replacePerm = 0o755

// ReplaceBinary atomically replaces the running binary with the file at
// newPath. It resolves the current executable through symlinks and replaces
// the RESOLVED target, so an install-time symlink itself is never clobbered.
// The new file is staged as a temp sibling in the same directory and renamed
// over the target; the temp file is removed on any failure. Unsupported on
// windows (ErrUnsupportedPlatform).
func ReplaceBinary(newPath string) error {
	if hostOS == "windows" {
		return ErrUnsupportedPlatform
	}

	target, err := resolvedSelfPath()
	if err != nil {
		return err
	}

	name := filepath.Base(target)
	tmpPath := filepath.Join(filepath.Dir(target), fmt.Sprintf(".%s.update-%d", name, os.Getpid()))

	in, err := os.Open(newPath)
	if err != nil {
		return fmt.Errorf("opening new binary %s: %w", filepath.Base(newPath), err)
	}
	defer func() { _ = in.Close() }()

	out, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, replacePerm)
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	discard := func(closer func() error) {
		_ = closer()
		_ = os.Remove(tmpPath)
	}
	if _, err := io.Copy(out, in); err != nil {
		discard(func() error { return out.Close() })
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("closing temp file: %w", err)
	}
	if err := os.Chmod(tmpPath, replacePerm); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("setting executable permissions: %w", err)
	}
	if err := os.Rename(tmpPath, target); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("replacing %s atomically: %w", name, err)
	}
	return nil
}
