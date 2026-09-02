package auth

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
	"golang.org/x/oauth2/google"

	"github.com/oskarhane/everything-cli/internal/output"
)

// CredentialsName is the credentials filename auto-detected in the config
// dir.
const CredentialsName = "credentials.json"

// ResolveCredentials returns the path of the OAuth app credentials JSON on fs.
//
// Order: the --credentials flag value, then <configDir>/credentials.json. A
// candidate is used only when it exists; when none exists, the returned
// error names every path tried.
//
// Deliberately NOT searched: the working directory. Credentials resolve
// ahead of every token refresh, so a credentials.json planted in the CWD
// would silently replace the OAuth app (and, before endpoint pinning, the
// token endpoints) on non-interactive commands. Keeping the search to the
// flag and the config dir means only locations the user (or the
// `account add` flow) explicitly controls can supply credentials.
func ResolveCredentials(fs afero.Fs, flagValue, configDir string) (string, error) {
	var tried []string
	exists := func(path string) bool {
		tried = append(tried, path)
		ok, err := afero.Exists(fs, path)
		return err == nil && ok
	}

	if flagValue != "" && exists(flagValue) {
		output.Debug("using credentials file " + flagValue)
		return flagValue, nil
	}
	if configDir != "" {
		if inConfig := filepath.Join(configDir, CredentialsName); exists(inConfig) {
			output.Debug("using credentials file " + inConfig)
			return inConfig, nil
		}
	}
	output.Debug("no credentials file found; tried " + strings.Join(tried, ", "))
	return "", fmt.Errorf("no OAuth credentials file found; tried: %s", strings.Join(tried, ", "))
}

// ClientCredentials carries an OAuth app's client_id and client_secret
// directly, so the flow and token machinery never needs a credentials
// document at runtime. Providers either parse their credentials file into
// this shape once (Google) or capture the values from flags/env (Linear).
type ClientCredentials struct {
	// ID is the OAuth app's client ID (non-secret metadata).
	ID string
	// Secret is the OAuth app's client secret; empty for public PKCE
	// clients. It is a secret: registered for redaction at mint/read.
	Secret string
}

// ParseClientCredentials parses installed-app credentials JSON into
// ClientCredentials. Only client_id/client_secret are taken from the
// document: auth_uri and token_uri are ignored (endpoints come from the
// pinned OAuthProfile), so a tampered or planted file can never redirect
// authorization or token (refresh) requests to another server. The client
// secret is registered for redaction at this read point.
func ParseClientCredentials(data []byte) (ClientCredentials, error) {
	conf, err := google.ConfigFromJSON(data)
	if err != nil {
		return ClientCredentials{}, err
	}
	if conf.ClientSecret != "" {
		RegisterSecret(conf.ClientSecret)
	}
	return ClientCredentials{ID: conf.ClientID, Secret: conf.ClientSecret}, nil
}

// ReadClientCredentials reads and parses the credentials file at path —
// the single read every credentials-consuming path funnels through.
func ReadClientCredentials(fs afero.Fs, path string) (ClientCredentials, error) {
	data, err := afero.ReadFile(fs, path)
	if err != nil {
		return ClientCredentials{}, fmt.Errorf("reading credentials %s: %w", path, err)
	}
	creds, err := ParseClientCredentials(data)
	if err != nil {
		return ClientCredentials{}, fmt.Errorf("parsing credentials %s: %w", path, err)
	}
	return creds, nil
}
