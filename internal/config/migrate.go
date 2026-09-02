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

// migrationMarker is written inside the new root while a legacy-dir
// migration is in progress. Its presence distinguishes a crashed migration
// (resume on the next run) from a pre-existing new dir that is the user's
// own (never touch). The marker is removed when the migration completes.
const migrationMarker = ".legacy-migration-in-progress"

// copyLegacyDir performs the first-run migration from the pre-rename config
// dir: when the default everything-cli dir does NOT exist yet but the
// sibling legacy google-cli dir DOES, the entire legacy tree is copied
// (never moved — the old dir is left intact) into the new dir.
//
// The migration is atomic per file and resumable across crashes. A marker
// file in the new root marks an in-progress migration; each file is copied
// temp+rename, so a crash mid-copy can never leave partial content at a
// target path, and the next run (marker still present) redoes only the
// files whose rename never landed. A new root that exists WITHOUT the
// marker is the user's own dir and is left alone.
//
// The store's private permission discipline is applied to every migrated
// entry rather than trusting source modes: directories land at 0700 and
// files at 0600, since the tree holds OAuth tokens. Symlinks and other
// non-regular files are skipped — the store replaces symlinks rather than
// following them, and a planted symlink in the legacy dir must not redirect
// writes outside the new dir.
func copyLegacyDir(fsys afero.Fs, newRoot string) error {
	legacyRoot := filepath.Join(filepath.Dir(newRoot), legacyDirName)
	markerPath := filepath.Join(newRoot, migrationMarker)

	if _, err := fsys.Stat(newRoot); err == nil {
		// The new dir exists: resume only a crashed migration (marker
		// present); any other pre-existing dir is the user's own — never
		// overwrite it.
		if _, err := fsys.Stat(markerPath); err == nil {
			// resume below
		} else if errors.Is(err, fs.ErrNotExist) {
			return nil
		} else {
			return fmt.Errorf("checking migration marker: %w", err)
		}
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

	if err := fsys.MkdirAll(newRoot, dirPermPrivate); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	// Best-effort marker write BEFORE any copy: a crash after this point
	// leaves a resumable state.
	if err := afero.WriteFile(fsys, markerPath, []byte("migration in progress\n"), filePermPrivate); err != nil {
		return fmt.Errorf("writing migration marker: %w", err)
	}

	err = afero.Walk(fsys, legacyRoot, func(path string, info fs.FileInfo, err error) error {
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
		// The rename is atomic, so an existing target is a complete copy
		// from earlier in this run or from the crashed one — skip it.
		if _, err := fsys.Stat(target); err == nil {
			return nil
		} else if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("checking migrated file: %w", err)
		}
		return copyLegacyFile(fsys, path, target)
	})
	if err != nil {
		return err
	}
	if err := fsys.Remove(markerPath); err != nil {
		return fmt.Errorf("clearing migration marker: %w", err)
	}
	return nil
}

// copyLegacyFile copies src to dst at 0600 via temp+rename, so a crash
// mid-copy never leaves partial content at dst. The temp name is
// deterministic (dst + ".tmp"): a crash leaves it behind and the resuming
// run truncates and redoes the copy, then renames it into place.
func copyLegacyFile(fsys afero.Fs, src, dst string) error {
	in, err := fsys.Open(src)
	if err != nil {
		return fmt.Errorf("reading legacy file: %w", err)
	}
	defer func() { _ = in.Close() }()

	tmp := dst + ".tmp"
	out, err := fsys.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, filePermPrivate)
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return fmt.Errorf("copying legacy file: %w", err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}
	if err := fsys.Chmod(tmp, filePermPrivate); err != nil {
		return fmt.Errorf("tightening temp file permissions: %w", err)
	}
	if err := fsys.Rename(tmp, dst); err != nil {
		return fmt.Errorf("placing migrated file: %w", err)
	}
	return nil
}
