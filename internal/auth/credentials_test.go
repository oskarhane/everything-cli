package auth

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	flagPath   = "/tmp/flag-credentials.json"
	configDirP = "/cfg"
	configPath = "/cfg/credentials.json"
	localPath  = "credentials.json"
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
			files: []string{flagPath, configPath, localPath},
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
			files: []string{configPath, localPath},
			want:  configPath,
		},
		{
			name:  "no flag, no config creds, working dir last",
			files: []string{localPath},
			want:  localPath,
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

func TestResolveCredentialsNoneExistErrors(t *testing.T) {
	fs := afero.NewMemMapFs()

	_, err := ResolveCredentials(fs, flagPath, configDirP)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no OAuth credentials")
}

// TestResolveCredentialsErrorNamesTriedPaths: the error must name every
// candidate path so the user knows where to put credentials.json.
func TestResolveCredentialsErrorNamesTriedPaths(t *testing.T) {
	fs := afero.NewMemMapFs()

	_, err := ResolveCredentials(fs, flagPath, configDirP)
	require.Error(t, err)
	for _, want := range []string{flagPath, configPath, localPath} {
		assert.Contains(t, err.Error(), want)
	}
}
