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
//
// Accounts live nested per provider at accounts/<provider>/<name>.json.
// The legacy pre-provider methods (List, Get, Remove, DefaultAccount,
// SetDefaultAccount, AccountPath) delegate to the google provider so
// existing callers compile and behave unchanged; legacy flat
// accounts/<name>.json files load as google accounts and are rewritten to
// the nested layout on the next Save.
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

// AccountPath returns the file backing the named Google account — the
// legacy spelling of AccountPathFor(ProviderGoogle, name).
func (s *Store) AccountPath(name string) string {
	return s.AccountPathFor(ProviderGoogle, name)
}

// AccountPathFor returns the file backing the named provider account:
// <configDir>/accounts/<provider>/<name>.json.
func (s *Store) AccountPathFor(provider, name string) string {
	return filepath.Join(s.providerDir(provider), name+".json")
}

// legacyAccountPath returns the pre-provider flat location of a Google
// account file: <configDir>/accounts/<name>.json.
func (s *Store) legacyAccountPath(name string) string {
	return filepath.Join(s.accountsDir(), name+".json")
}

func (s *Store) accountsDir() string { return filepath.Join(s.root, "accounts") }

func (s *Store) providerDir(provider string) string {
	return filepath.Join(s.accountsDir(), provider)
}

// List returns all persisted Google accounts sorted by name — the legacy
// spelling of ListProvider(ProviderGoogle).
func (s *Store) List() ([]Account, error) {
	return s.ListProvider(ProviderGoogle)
}

// ListProvider returns the provider's persisted accounts sorted by name. A
// provider with no accounts yet lists nothing without error. Google
// additionally picks up legacy flat accounts/<name>.json files not yet
// rewritten to the nested layout; a name present in both resolves to the
// nested file.
func (s *Store) ListProvider(provider string) ([]Account, error) {
	if err := validateProvider(provider); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var names []string
	collect := func(dir string) error {
		entries, err := afero.ReadDir(s.fs, dir)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return fmt.Errorf("listing accounts: %w", err)
		}
		for _, e := range entries {
			if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
				continue
			}
			name := strings.TrimSuffix(e.Name(), ".json")
			if seen[name] {
				continue
			}
			seen[name] = true
			names = append(names, name)
		}
		return nil
	}
	if err := collect(s.providerDir(provider)); err != nil {
		return nil, err
	}
	if provider == ProviderGoogle {
		if err := collect(s.accountsDir()); err != nil {
			return nil, err
		}
	}
	accounts := make([]Account, 0, len(names))
	for _, name := range names {
		a, err := s.GetProvider(provider, name)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, *a)
	}
	sort.Slice(accounts, func(i, j int) bool { return accounts[i].Name < accounts[j].Name })
	return accounts, nil
}

// ListAll returns every account of every provider, sorted by provider then
// name — the aggregate behind the read-only top-level account list.
// A store with no accounts yet lists nothing without error.
func (s *Store) ListAll() ([]Account, error) {
	entries, err := afero.ReadDir(s.fs, s.accountsDir())
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("listing accounts: %w", err)
	}
	// Google is always probed: legacy flat files may exist without a
	// google/ subdirectory.
	providers := map[string]bool{ProviderGoogle: true}
	for _, e := range entries {
		if e.IsDir() && validProviderID(e.Name()) {
			providers[e.Name()] = true
		}
	}
	var all []Account
	for provider := range providers {
		accounts, err := s.ListProvider(provider)
		if err != nil {
			return nil, err
		}
		all = append(all, accounts...)
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].Provider != all[j].Provider {
			return all[i].Provider < all[j].Provider
		}
		return all[i].Name < all[j].Name
	})
	return all, nil
}

// Get returns the named Google account — the legacy spelling of
// GetProvider(ProviderGoogle, name).
func (s *Store) Get(name string) (*Account, error) {
	return s.GetProvider(ProviderGoogle, name)
}

// GetProvider returns the named provider account. For google, a missing
// nested file falls back to the legacy flat accounts/<name>.json. Files
// written before the provider field existed load as accounts of the
// provider they were read under.
func (s *Store) GetProvider(provider, name string) (*Account, error) {
	if err := validateProvider(provider); err != nil {
		return nil, err
	}
	if !validAccountName(name) {
		return nil, fmt.Errorf("invalid account name %q", name)
	}
	data, err := afero.ReadFile(s.fs, s.AccountPathFor(provider, name))
	if err != nil && provider == ProviderGoogle && errors.Is(err, fs.ErrNotExist) {
		data, err = afero.ReadFile(s.fs, s.legacyAccountPath(name))
	}
	if err != nil {
		return nil, fmt.Errorf("reading account %q: %w", name, err)
	}
	var a Account
	if err := json.Unmarshal(data, &a); err != nil {
		return nil, fmt.Errorf("parsing account %q: %w", name, err)
	}
	if a.Provider == "" {
		a.Provider = provider
	}
	return &a, nil
}

// Save persists acct as <configDir>/accounts/<provider>/<name>.json with
// 0600 permissions, atomically replacing any existing file (and any symlink
// at that path, which is replaced rather than followed). An empty
// acct.Provider means google.
//
// Accounts are deduplicated by email within their provider: when an account
// with the same non-empty email already exists under a different name, that
// record is updated under its original name and acct.Name is rewritten to
// it, so no duplicate account is created.
//
// A google save also removes any legacy flat accounts/<name>.json for the
// saved name(s), completing the migration to the nested layout. When the
// provider has no default account yet, the saved account becomes it.
func (s *Store) Save(acct *Account) error {
	if acct.Provider == "" {
		acct.Provider = ProviderGoogle
	}
	if err := validateProvider(acct.Provider); err != nil {
		return err
	}
	if !validAccountName(acct.Name) {
		return fmt.Errorf("invalid account name %q", acct.Name)
	}
	origName := acct.Name
	if acct.Email != "" {
		existing, err := s.findByEmail(acct.Provider, acct.Email)
		if err != nil {
			return err
		}
		if existing != "" && existing != acct.Name {
			acct.Name = existing
		}
	}
	if err := s.fs.MkdirAll(s.providerDir(acct.Provider), dirPermPrivate); err != nil {
		return fmt.Errorf("creating accounts dir: %w", err)
	}
	if err := s.hardenDir(s.root); err != nil {
		return fmt.Errorf("tightening config dir permissions: %w", err)
	}
	if err := s.hardenDir(s.accountsDir()); err != nil {
		return fmt.Errorf("tightening accounts dir permissions: %w", err)
	}
	if err := s.hardenDir(s.providerDir(acct.Provider)); err != nil {
		return fmt.Errorf("tightening provider dir permissions: %w", err)
	}
	data, err := json.MarshalIndent(acct, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding account %q: %w", acct.Name, err)
	}
	data = append(data, '\n')
	if err := s.writePrivate(s.AccountPathFor(acct.Provider, acct.Name), data); err != nil {
		return fmt.Errorf("writing account %q: %w", acct.Name, err)
	}
	if acct.Provider == ProviderGoogle {
		// Rewrite legacy flat files to the nested layout: Remove unlinks
		// the flat file (or a symlink at that path — Remove never follows
		// it) now that the nested file holds the account.
		for _, name := range []string{origName, acct.Name} {
			if err := s.fs.Remove(s.legacyAccountPath(name)); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("removing legacy account file %q: %w", name, err)
			}
		}
	}
	def, err := s.DefaultAccountFor(acct.Provider)
	if err != nil {
		return err
	}
	if def == "" {
		if err := s.writeDefault(acct.Provider, acct.Name); err != nil {
			return err
		}
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

// Remove deletes the named Google account, clearing it as the default if it
// was. This legacy spelling keeps the pre-provider default policy — clear,
// never promote — so existing callers behave unchanged; new code should use
// RemoveProvider, which auto-manages the provider default.
func (s *Store) Remove(name string) error {
	return s.remove(ProviderGoogle, name, false)
}

// RemoveProvider deletes the named provider account, from the nested file
// and — for google — any legacy flat file. When the removed account was the
// provider's default, another account of that provider is promoted (the
// first by name, deterministically); the default is cleared when none
// remain.
func (s *Store) RemoveProvider(provider, name string) error {
	return s.remove(provider, name, true)
}

func (s *Store) remove(provider, name string, promote bool) error {
	if err := validateProvider(provider); err != nil {
		return err
	}
	if !validAccountName(name) {
		return fmt.Errorf("invalid account name %q", name)
	}
	removed := false
	if err := s.fs.Remove(s.AccountPathFor(provider, name)); err == nil {
		removed = true
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("removing account %q: %w", name, err)
	}
	if provider == ProviderGoogle {
		if err := s.fs.Remove(s.legacyAccountPath(name)); err == nil {
			removed = true
		} else if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("removing account %q: %w", name, err)
		}
	}
	if !removed {
		return fmt.Errorf("removing account %q: %w", name, fs.ErrNotExist)
	}
	def, err := s.DefaultAccountFor(provider)
	if err != nil {
		return err
	}
	if def != name {
		return nil
	}
	if promote {
		accounts, err := s.ListProvider(provider)
		if err != nil {
			return err
		}
		if len(accounts) > 0 {
			return s.writeDefault(provider, accounts[0].Name)
		}
	}
	return s.clearDefault(provider)
}

// findByEmail returns the name of the provider's account with the given
// email, or "".
func (s *Store) findByEmail(provider, email string) (string, error) {
	accounts, err := s.ListProvider(provider)
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

// validateProvider rejects provider IDs that would escape the accounts dir.
func validateProvider(provider string) error {
	if !validProviderID(provider) {
		return fmt.Errorf("invalid provider %q", provider)
	}
	return nil
}

// validProviderID applies the account-name rules to provider IDs: a
// provider ID becomes a path segment under the accounts dir, so the same
// escapes and control bytes must be rejected.
func validProviderID(provider string) bool {
	return validAccountName(provider)
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
