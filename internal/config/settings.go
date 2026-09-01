package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/spf13/afero"
)

// settings is the <configDir>/config.json document. Defaults are
// per-provider; default_account is the legacy pre-provider spelling of
// default_accounts.google and is migrated on load.
type settings struct {
	DefaultAccount  string            `json:"default_account,omitempty"`
	DefaultAccounts map[string]string `json:"default_accounts,omitempty"`
}

// normalize folds the legacy default_account key into the per-provider map
// so every reader (and the next writer) sees the migrated shape.
func (c *settings) normalize() {
	if c.DefaultAccounts == nil {
		c.DefaultAccounts = map[string]string{}
	}
	if c.DefaultAccount != "" {
		if _, ok := c.DefaultAccounts[ProviderGoogle]; !ok {
			c.DefaultAccounts[ProviderGoogle] = c.DefaultAccount
		}
		c.DefaultAccount = ""
	}
}

func (s *Store) settingsPath() string { return filepath.Join(s.root, "config.json") }

// loadSettings reads config.json, transparently migrating a legacy
// default_account key to default_accounts.google. A missing file is an
// empty settings document, not an error.
func (s *Store) loadSettings() (*settings, error) {
	data, err := afero.ReadFile(s.fs, s.settingsPath())
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			cfg := &settings{}
			cfg.normalize()
			return cfg, nil
		}
		return nil, fmt.Errorf("reading settings: %w", err)
	}
	var cfg settings
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing settings: %w", err)
	}
	cfg.normalize()
	return &cfg, nil
}

// DefaultAccount returns the default Google account name, or "" when unset
// — the legacy spelling of DefaultAccountFor(ProviderGoogle).
func (s *Store) DefaultAccount() (string, error) {
	return s.DefaultAccountFor(ProviderGoogle)
}

// DefaultAccountFor returns the provider's default account name, or "" when
// unset.
func (s *Store) DefaultAccountFor(provider string) (string, error) {
	cfg, err := s.loadSettings()
	if err != nil {
		return "", err
	}
	return cfg.DefaultAccounts[provider], nil
}

// SetDefaultAccount persists name as the default Google account — the
// legacy spelling of SetDefaultAccountFor(ProviderGoogle, name).
func (s *Store) SetDefaultAccount(name string) error {
	return s.SetDefaultAccountFor(ProviderGoogle, name)
}

// SetDefaultAccountFor persists name as the provider's default account,
// preserving the other providers' defaults. The account must exist, so no
// dangling default can be recorded.
func (s *Store) SetDefaultAccountFor(provider, name string) error {
	if _, err := s.GetProvider(provider, name); err != nil {
		return err
	}
	return s.writeDefault(provider, name)
}

// writeDefault records name as the provider's default.
func (s *Store) writeDefault(provider, name string) error {
	if err := s.fs.MkdirAll(s.root, dirPermPrivate); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	if err := s.hardenDir(s.root); err != nil {
		return fmt.Errorf("tightening config dir permissions: %w", err)
	}
	cfg, err := s.loadSettings()
	if err != nil {
		return err
	}
	cfg.DefaultAccounts[provider] = name
	return s.writeSettings(cfg)
}

// clearDefault removes the provider's default, dropping the settings file
// entirely when no provider default remains.
func (s *Store) clearDefault(provider string) error {
	cfg, err := s.loadSettings()
	if err != nil {
		return err
	}
	delete(cfg.DefaultAccounts, provider)
	return s.writeSettings(cfg)
}

func (s *Store) writeSettings(cfg *settings) error {
	if len(cfg.DefaultAccounts) == 0 {
		if err := s.fs.Remove(s.settingsPath()); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("clearing default account: %w", err)
		}
		return nil
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding settings: %w", err)
	}
	data = append(data, '\n')
	if err := s.writePrivate(s.settingsPath(), data); err != nil {
		return fmt.Errorf("writing settings: %w", err)
	}
	return nil
}
