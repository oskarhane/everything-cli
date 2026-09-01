package config

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/afero"
)

// copyLegacyDir performs the first-run migration from the pre-rename config
// dir: when the default everything-cli dir does NOT exist yet but the
// sibling legacy google-cli dir DOES, the entire legacy tree is copied
// (never moved — the old dir is left intact) into the new dir.
//
// The store's private permission discipline is applied to every migrated
// entry rather than trusting source modes: directories land at 0700 and
// files at 0600, since the tree holds OAuth tokens. Symlinks and other
// non-regular files are skipped — the store replaces symlinks rather than
// following them, and a planted symlink in the legacy dir must not redirect
// writes outside the new dir.
func copyLegacyDir(fsys afero.Fs, newRoot string) error {
	legacyRoot := filepath.Join(filepath.Dir(newRoot), legacyDirName)

	if _, err := fsys.Stat(newRoot); err == nil {
		return nil // new dir already exists — never overwrite
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("checking config dir: %w", err)
	}
	info, err := fsys.Stat(legacyRoot)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil // no legacy dir — nothing to migrate
		}
		return fmt.Errorf("checking legacy config dir: %w", err)
	}
	if !info.IsDir() {
		return nil
	}

	return afero.Walk(fsys, legacyRoot, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(legacyRoot, path)
		if err != nil {
			return fmt.Errorf("resolving legacy path: %w", err)
		}
		target := filepath.Join(newRoot, rel)
		if info.IsDir() {
			if err := fsys.MkdirAll(target, dirPermPrivate); err != nil {
				return fmt.Errorf("creating migrated dir: %w", err)
			}
			// MkdirAll is a no-op on existing dirs, so chmod explicitly to
			// guarantee 0700 even when a wider parent pre-existed.
			if err := fsys.Chmod(target, dirPermPrivate); err != nil {
				return fmt.Errorf("tightening migrated dir permissions: %w", err)
			}
			return nil
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		src, err := fsys.Open(path)
		if err != nil {
			return fmt.Errorf("reading legacy file: %w", err)
		}
		// O_EXCL: the new dir was absent above, so any existing target is
		// unexpected and must not be overwritten.
		dst, err := fsys.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, filePermPrivate)
		if err != nil {
			_ = src.Close()
			return fmt.Errorf("creating migrated file: %w", err)
		}
		if _, err := io.Copy(dst, src); err != nil {
			_ = src.Close()
			_ = dst.Close()
			return fmt.Errorf("copying legacy file: %w", err)
		}
		if err := src.Close(); err != nil {
			_ = dst.Close()
			return fmt.Errorf("closing legacy file: %w", err)
		}
		if err := dst.Close(); err != nil {
			return fmt.Errorf("closing migrated file: %w", err)
		}
		if err := fsys.Chmod(target, filePermPrivate); err != nil {
			return fmt.Errorf("tightening migrated file permissions: %w", err)
		}
		return nil
	})
}
