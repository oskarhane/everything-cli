package auth

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"

	"github.com/oskarhane/google-cli/internal/output"
)

// CredentialsName is the credentials filename auto-detected in the config
// dir and in the working directory.
const CredentialsName = "credentials.json"

// ResolveCredentials returns the path of the OAuth app credentials JSON on fs.
//
// Order: the --credentials flag value, then <configDir>/credentials.json,
// then ./credentials.json. A candidate is used only when it exists; when
// none exists, the returned error names every path tried.
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
	if inConfig := filepath.Join(configDir, CredentialsName); exists(inConfig) {
		output.Debug("using credentials file " + inConfig)
		return inConfig, nil
	}
	if exists(CredentialsName) {
		output.Debug("using credentials file " + CredentialsName)
		return CredentialsName, nil
	}
	output.Debug("no credentials file found; tried " + strings.Join(tried, ", "))
	return "", fmt.Errorf("no OAuth credentials file found; tried: %s", strings.Join(tried, ", "))
}
