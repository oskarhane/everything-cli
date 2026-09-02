package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/afero"
	"golang.org/x/oauth2"
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
	fs afero.Fs
	// envRoot is true when root came from $EVERYTHING_CLI_CONFIG_DIR (or
	// its legacy spelling): the user deliberately points at that dir, so a
	// pre-existing env root's permissions are their choice and must not be
	// chmodded — only dirs the store creates are tightened.
	envRoot bool
	root    string
}

// NewStore returns a Store on fs rooted at dir. An empty dir resolves via
// ResolveDir: $EVERYTHING_CLI_CONFIG_DIR, then $GOOGLE_CLI_CONFIG_DIR
// (deprecated), then ~/.config/everything-cli.
//
// When resolution lands on the default dir (no explicit dir, no env
// override) and it does not exist yet, a legacy ~/.config/google-cli tree
// is copied over first, so existing accounts survive the rename with no
// user action. The legacy dir is left intact.
func NewStore(fs afero.Fs, dir string) (*Store, error) {
	root, source, err := resolveDir(dir)
	if err != nil {
		return nil, err
	}
	if source == sourceDefault {
		if err := copyLegacyDir(fs, root); err != nil {
			return nil, fmt.Errorf("migrating legacy config dir: %w", err)
		}
	}
	return &Store{fs: fs, root: root, envRoot: source == sourceEnv}, nil
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
// saved name(s), completing the migration to the nested layout — unless the
// flat record belongs to a different identity (different non-empty email),
// in which case it is left intact. When the provider has no default account
// yet, the saved account becomes it.
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
	// Harden before MkdirAll: hardenDir replaces a symlinked dir with a real
	// one, and MkdirAll must not follow a planted symlink out of the config
	// dir.
	if err := s.hardenRoot(); err != nil {
		return fmt.Errorf("tightening config dir permissions: %w", err)
	}
	if err := s.hardenDir(s.accountsDir()); err != nil {
		return fmt.Errorf("tightening accounts dir permissions: %w", err)
	}
	if err := s.fs.MkdirAll(s.providerDir(acct.Provider), dirPermPrivate); err != nil {
		return fmt.Errorf("creating accounts dir: %w", err)
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
		// Rewrite legacy flat files to the nested layout: the flat file (or
		// a symlink at that path — Remove never follows it) is unlinked now
		// that the nested file holds the account, unless the flat record
		// belongs to a different identity.
		for _, name := range []string{origName, acct.Name} {
			if err := s.removeLegacyFile(name, acct.Email); err != nil {
				return err
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

// SaveToken replaces the token of an existing provider account, writing
// only the account file: unlike Save it performs no email dedup and never
// touches the provider's default account. It is the persist path for
// background token refreshes — a refresh must not silently switch which
// account bare commands resolve to by re-running Save's default
// management. An empty provider means google.
func (s *Store) SaveToken(provider, name string, tok *oauth2.Token) error {
	if provider == "" {
		provider = ProviderGoogle
	}
	acct, err := s.GetProvider(provider, name) // validates provider and name
	if err != nil {
		return err
	}
	acct.Token = tok
	if err := s.fs.MkdirAll(s.providerDir(provider), dirPermPrivate); err != nil {
		return fmt.Errorf("creating accounts dir: %w", err)
	}
	data, err := json.MarshalIndent(acct, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding account %q: %w", name, err)
	}
	data = append(data, '\n')
	if err := s.writePrivate(s.AccountPathFor(provider, name), data); err != nil {
		return fmt.Errorf("writing account %q: %w", name, err)
	}
	if provider == ProviderGoogle {
		// Keep Save's migration: once the nested file holds the account,
		// the legacy flat file is unlinked unless it belongs to a
		// different identity.
		if err := s.removeLegacyFile(name, acct.Email); err != nil {
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

// removeLegacyFile unlinks the legacy flat accounts/<name>.json now that
// the nested file holds the account. The flat record is loaded first: when
// it parses and carries a different non-empty email than the saved account,
// it is an un-migrated account of a DIFFERENT identity that happens to share
// the name — deleting it would destroy that identity's only copy, so it is
// left intact. (A legacy account of the same identity surfaces through
// Save's findByEmail dedup, which reuses the flat record's name, so the
// emails match on that path.)
func (s *Store) removeLegacyFile(name, email string) error {
	path := s.legacyAccountPath(name)
	data, err := afero.ReadFile(s.fs, path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("reading legacy account file %q: %w", name, err)
	}
	var legacy Account
	if err := json.Unmarshal(data, &legacy); err == nil &&
		legacy.Email != "" && legacy.Email != email {
		return nil // different identity — never destroy its un-migrated record
	}
	if err := s.fs.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("removing legacy account file %q: %w", name, err)
	}
	return nil
}

// hardenRoot tightens the config root like hardenDir — except for an
// env-pointed root: the user deliberately shared that dir via
// $EVERYTHING_CLI_CONFIG_DIR, so chmodding a pre-existing env root to 0700
// could break whatever else they keep there. Roots the store creates land
// at 0700 via MkdirAll, so skipping the chmod loses nothing.
func (s *Store) hardenRoot() error {
	if s.envRoot {
		return nil
	}
	return s.hardenDir(s.root)
}

// hardenDir tightens an existing directory to 0700 when it pre-exists wider.
// Missing directories are left to the caller's MkdirAll.
//
// The path is Lstat'd, never followed: a symlink planted at the path would
// otherwise redirect the chmod (and later writes) to the link target
// outside the config dir. A symlink is unlinked and recreated as a real
// private directory — the link itself is removed, never its target.
func (s *Store) hardenDir(path string) error {
	info, err := lstat(s.fs, path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		if err := s.fs.Remove(path); err != nil {
			return fmt.Errorf("unlinking symlinked dir %s: %w", path, err)
		}
		if err := s.fs.MkdirAll(path, dirPermPrivate); err != nil {
			return fmt.Errorf("recreating symlinked dir %s: %w", path, err)
		}
		return nil
	}
	if info.IsDir() && info.Mode().Perm() != dirPermPrivate {
		if err := s.fs.Chmod(path, dirPermPrivate); err != nil {
			return fmt.Errorf("chmod %s: %w", path, err)
		}
	}
	return nil
}

// lstat stats path without following a trailing symlink when the
// filesystem supports it, falling back to Stat otherwise.
func lstat(fsys afero.Fs, path string) (fs.FileInfo, error) {
	if l, ok := fsys.(afero.Lstater); ok {
		info, _, err := l.LstatIfPossible(path)
		return info, err
	}
	return fsys.Stat(path)
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
