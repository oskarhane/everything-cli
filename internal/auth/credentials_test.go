package auth

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2/google"
)

const (
	flagPath   = "/tmp/flag-credentials.json"
	configDirP = "/cfg"
	configPath = "/cfg/credentials.json"
	// localPath is the CWD-planted file the refresh path must ignore.
	localPath = "credentials.json"
)

func TestResolveCredentials(t *testing.T) {
	tests := []struct {
		name  string
		flag  string
		files []string
		want  string
	}{
		{
			name:  "flag wins when it exists",
			flag:  flagPath,
			files: []string{flagPath, configPath},
			want:  flagPath,
		},
		{
			name:  "missing flag falls through to config dir",
			flag:  "/tmp/does-not-exist.json",
			files: []string{configPath},
			want:  configPath,
		},
		{
			name:  "no flag, config dir next",
			files: []string{configPath},
			want:  configPath,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := afero.NewMemMapFs()
			for _, f := range tt.files {
				require.NoError(t, afero.WriteFile(fs, f, []byte("{}"), 0o600))
			}

			got, err := ResolveCredentials(fs, tt.flag, configDirP)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestResolveCredentialsNeverUsesCWD: a credentials.json in the working
// directory must never be picked up, no matter what else is missing — the
// resolved file feeds every token refresh, so a planted CWD file would
// replace the OAuth app on non-interactive commands.
func TestResolveCredentialsNeverUsesCWD(t *testing.T) {
	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, localPath, []byte("{}"), 0o600))

	_, err := ResolveCredentials(fs, "", configDirP)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no OAuth credentials")
	assert.NotContains(t, err.Error(), "./"+localPath,
		"the CWD must never appear among the tried paths")
}

// TestResolveCredentialsRefreshPathCannotUsePlantedCWD proves the refresh
// chain (Dial -> ResolveCredentials -> TokenSource) cannot be redirected by
// a CWD-planted credentials file: with no --credentials flag and no
// credentials in the config dir, resolution errors even though
// ./credentials.json exists, so no refresh endpoint can come from it.
func TestResolveCredentialsRefreshPathCannotUsePlantedCWDFile(t *testing.T) {
	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, localPath,
		[]byte(`{"installed":{"client_id":"attacker","token_uri":"https://evil.example/token"}}`), 0o600))

	_, err := ResolveCredentials(fs, "", configDirP)
	require.Error(t, err, "a CWD-planted credentials file must not satisfy the refresh path")
	assert.Contains(t, err.Error(), configPath, "error must name the config-dir path tried")
}

func TestResolveCredentialsNoneExistErrors(t *testing.T) {
	fs := afero.NewMemMapFs()

	_, err := ResolveCredentials(fs, flagPath, configDirP)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no OAuth credentials")
}

// TestResolveCredentialsErrorNamesTriedPaths: the error must name every
// candidate path so the user knows where to put credentials.json. The two
// candidates are the --credentials flag and the config dir; the CWD is
// deliberately never tried.
func TestResolveCredentialsErrorNamesTriedPaths(t *testing.T) {
	fs := afero.NewMemMapFs()

	_, err := ResolveCredentials(fs, flagPath, configDirP)
	require.Error(t, err)
	for _, want := range []string{flagPath, configPath} {
		assert.Contains(t, err.Error(), want)
	}
}

// TestGoogleConfigPinsEndpoints: auth_uri/token_uri from the credentials
// file are ignored — the file parses into ClientCredentials (which carry no
// endpoints), and the config built from them always targets the profile's
// pinned Google endpoints, so a tampered file cannot redirect token
// requests.
func TestGoogleConfigPinsEndpoints(t *testing.T) {
	data := []byte(`{
	  "installed": {
	    "client_id": "test-client-id",
	    "client_secret": "test-client-secret",
	    "auth_uri": "https://evil.example/auth",
	    "token_uri": "https://evil.example/token",
	    "redirect_uris": ["http://localhost"]
	  }
	}`)

	creds, err := ParseClientCredentials(data)
	require.NoError(t, err)
	assert.Equal(t, "test-client-id", creds.ID,
		"parsing must keep the file's client_id")

	conf := oauthConfigFor(GoogleOAuth, creds, "scope-a")
	assert.Equal(t, google.Endpoint.AuthURL, conf.Endpoint.AuthURL)
	assert.Equal(t, google.Endpoint.TokenURL, conf.Endpoint.TokenURL)
	assert.Equal(t, "test-client-id", conf.ClientID)
	assert.Equal(t, []string{"scope-a"}, conf.Scopes)
}
