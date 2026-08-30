package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/afero"
)

// filePermPrivate is the permission for files under the config dir:
// account files hold OAuth tokens, so everything there is private.
const filePermPrivate fs.FileMode = 0o600

// dirPermPrivate is the permission for the config and accounts directories.
const dirPermPrivate fs.FileMode = 0o700

// Store persists accounts and settings under the config directory on an
// injectable afero filesystem.
type Store struct {
	fs   afero.Fs
	root string
}

// NewStore returns a Store on fs rooted at dir. An empty dir resolves via
// ResolveDir: $GOOGLE_CLI_CONFIG_DIR, then ~/.config/google-cli.
func NewStore(fs afero.Fs, dir string) (*Store, error) {
	root, err := ResolveDir(dir)
	if err != nil {
		return nil, err
	}
	return &Store{fs: fs, root: root}, nil
}

// Dir returns the resolved config directory.
func (s *Store) Dir() string { return s.root }

// CredentialsPath returns the conventional credentials.json location inside
// the config dir, used by auth credentials resolution.
func (s *Store) CredentialsPath() string {
	return filepath.Join(s.root, "credentials.json")
}

// AccountPath returns the file backing the named account.
func (s *Store) AccountPath(name string) string {
	return filepath.Join(s.root, "accounts", name+".json")
}

func (s *Store) accountsDir() string { return filepath.Join(s.root, "accounts") }

func (s *Store) settingsPath() string { return filepath.Join(s.root, "config.json") }

// List returns all persisted accounts sorted by name. A store with no
// accounts yet lists nothing without error.
func (s *Store) List() ([]Account, error) {
	entries, err := afero.ReadDir(s.fs, s.accountsDir())
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("listing accounts: %w", err)
	}
	accounts := make([]Account, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		a, err := s.Get(strings.TrimSuffix(e.Name(), ".json"))
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, *a)
	}
	sort.Slice(accounts, func(i, j int) bool { return accounts[i].Name < accounts[j].Name })
	return accounts, nil
}

// Get returns the named account.
func (s *Store) Get(name string) (*Account, error) {
	if !validAccountName(name) {
		return nil, fmt.Errorf("invalid account name %q", name)
	}
	data, err := afero.ReadFile(s.fs, s.AccountPath(name))
	if err != nil {
		return nil, fmt.Errorf("reading account %q: %w", name, err)
	}
	var a Account
	if err := json.Unmarshal(data, &a); err != nil {
		return nil, fmt.Errorf("parsing account %q: %w", name, err)
	}
	return &a, nil
}

// Save persists acct as <configDir>/accounts/<name>.json with 0600
// permissions, atomically replacing any existing file (and any symlink at
// that path, which is replaced rather than followed).
//
// Accounts are deduplicated by email: when an account with the same email
// already exists under a different name, that record is updated under its
// original name and acct.Name is rewritten to it, so no duplicate account is
// created.
func (s *Store) Save(acct *Account) error {
	if !validAccountName(acct.Name) {
		return fmt.Errorf("invalid account name %q", acct.Name)
	}
	existing, err := s.findByEmail(acct.Email)
	if err != nil {
		return err
	}
	if existing != "" && existing != acct.Name {
		acct.Name = existing
	}
	if err := s.fs.MkdirAll(s.accountsDir(), dirPermPrivate); err != nil {
		return fmt.Errorf("creating accounts dir: %w", err)
	}
	if err := s.hardenDir(s.root); err != nil {
		return fmt.Errorf("tightening config dir permissions: %w", err)
	}
	if err := s.hardenDir(s.accountsDir()); err != nil {
		return fmt.Errorf("tightening accounts dir permissions: %w", err)
	}
	data, err := json.MarshalIndent(acct, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding account %q: %w", acct.Name, err)
	}
	data = append(data, '\n')
	if err := s.writePrivate(s.AccountPath(acct.Name), data); err != nil {
		return fmt.Errorf("writing account %q: %w", acct.Name, err)
	}
	return nil
}

// writePrivate atomically replaces path with data at 0600. It writes to a
// temp file in the same directory (0600), then renames over the target, so:
//
//   - a crash mid-write can never leave partial JSON at the target path, and
//   - a symlink at path is replaced rather than followed (rename does not
//     traverse symlinks on the destination).
//
// The rename is the durability point; the temp file is removed on any
// earlier failure.
func (s *Store) writePrivate(path string, data []byte) error {
	tmp, err := afero.TempFile(s.fs, filepath.Dir(path), "."+filepath.Base(path)+".tmp-")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmp.Name()
	discard := func() {
		_ = tmp.Close()
		_ = s.fs.Remove(tmpPath)
	}
	if _, err := tmp.Write(data); err != nil {
		discard()
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = s.fs.Remove(tmpPath)
		return fmt.Errorf("closing temp file: %w", err)
	}
	if err := s.fs.Chmod(tmpPath, filePermPrivate); err != nil {
		_ = s.fs.Remove(tmpPath)
		return fmt.Errorf("setting private permissions: %w", err)
	}
	if err := s.fs.Rename(tmpPath, path); err != nil {
		_ = s.fs.Remove(tmpPath)
		return fmt.Errorf("replacing %s atomically: %w", filepath.Base(path), err)
	}
	// Rename carried the temp file's mode, but tighten explicitly so a
	// non-conforming fs cannot land wider perms at the target.
	if err := s.fs.Chmod(path, filePermPrivate); err != nil {
		return fmt.Errorf("setting private permissions: %w", err)
	}
	return nil
}

// hardenDir tightens an existing directory to 0700 when it pre-exists wider.
// Missing directories are left to the caller's MkdirAll.
func (s *Store) hardenDir(path string) error {
	info, err := s.fs.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	if info.IsDir() && info.Mode().Perm() != dirPermPrivate {
		if err := s.fs.Chmod(path, dirPermPrivate); err != nil {
			return fmt.Errorf("chmod %s: %w", path, err)
		}
	}
	return nil
}

// Remove deletes the named account, clearing it as the default if it was.
func (s *Store) Remove(name string) error {
	if !validAccountName(name) {
		return fmt.Errorf("invalid account name %q", name)
	}
	if err := s.fs.Remove(s.AccountPath(name)); err != nil {
		return fmt.Errorf("removing account %q: %w", name, err)
	}
	if def, err := s.DefaultAccount(); err == nil && def == name {
		if err := s.clearDefault(); err != nil {
			return err
		}
	}
	return nil
}

// DefaultAccount returns the default account name, or "" when unset.
func (s *Store) DefaultAccount() (string, error) {
	data, err := afero.ReadFile(s.fs, s.settingsPath())
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("reading settings: %w", err)
	}
	var cfg settings
	if err := json.Unmarshal(data, &cfg); err != nil {
		return "", fmt.Errorf("parsing settings: %w", err)
	}
	return cfg.DefaultAccount, nil
}

// SetDefaultAccount persists name as the default account. The account must
// exist, so no dangling default can be recorded.
func (s *Store) SetDefaultAccount(name string) error {
	if _, err := s.Get(name); err != nil {
		return err
	}
	if err := s.fs.MkdirAll(s.root, dirPermPrivate); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	if err := s.hardenDir(s.root); err != nil {
		return fmt.Errorf("tightening config dir permissions: %w", err)
	}
	data, err := json.MarshalIndent(settings{DefaultAccount: name}, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding settings: %w", err)
	}
	data = append(data, '\n')
	if err := s.writePrivate(s.settingsPath(), data); err != nil {
		return fmt.Errorf("writing settings: %w", err)
	}
	return nil
}

// settings is the <configDir>/config.json document.
type settings struct {
	DefaultAccount string `json:"default_account"`
}

func (s *Store) clearDefault() error {
	if err := s.fs.Remove(s.settingsPath()); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("clearing default account: %w", err)
	}
	return nil
}

// findByEmail returns the name of the account with the given email, or "".
func (s *Store) findByEmail(email string) (string, error) {
	accounts, err := s.List()
	if err != nil {
		return "", err
	}
	for _, a := range accounts {
		if a.Email == email {
			return a.Name, nil
		}
	}
	return "", nil
}

// validAccountName rejects names that would escape the accounts dir, names
// that would smuggle characters past filename parsing (':' on some systems,
// NUL), and unprintable control bytes (C0 and DEL).
func validAccountName(name string) bool {
	switch name {
	case "", ".", "..":
		return false
	}
	if strings.ContainsAny(name, `/\:`) {
		return false
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7F {
			return false
		}
	}
	return true
}
