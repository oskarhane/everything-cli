package auth

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/oskarhane/google-cli/internal/output"
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

// parseGoogleCredentials parses installed-app credentials JSON into an OAuth
// config with the endpoints pinned to Google's. The file therefore supplies
// only client_id/client_secret: auth_uri and token_uri from a credentials
// file are ignored, so a tampered or planted file can never redirect
// authorization or token (refresh) requests to another server.
func parseGoogleCredentials(data []byte, scopes ...string) (*oauth2.Config, error) {
	conf, err := google.ConfigFromJSON(data, scopes...)
	if err != nil {
		return nil, err
	}
	conf.Endpoint = google.Endpoint
	return conf, nil
}
