package config

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureStderr swaps the package stderr for a buffer and restores it on
// test cleanup.
func captureStderr(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	orig := stderr
	stderr = &buf
	t.Cleanup(func() { stderr = orig })
	return &buf
}

func TestResolveDir(t *testing.T) {
	home := t.TempDir()

	tests := []struct {
		name        string
		root        string
		newEnv      *string
		legacyEnv   *string
		want        string
		wantWarning bool
	}{
		{
			name:      "explicit root wins over both env vars",
			root:      "/explicit/root",
			newEnv:    ptr("/new/env"),
			legacyEnv: ptr("/legacy/env"),
			want:      "/explicit/root",
		},
		{
			name:      "new env var wins over legacy",
			newEnv:    ptr("/new/env"),
			legacyEnv: ptr("/legacy/env"),
			want:      "/new/env",
		},
		{
			name:   "new env var alone",
			newEnv: ptr("/new/env"),
			want:   "/new/env",
		},
		{
			name:        "legacy env var honored with deprecation warning",
			legacyEnv:   ptr("/legacy/env"),
			want:        "/legacy/env",
			wantWarning: true,
		},
		{
			name:        "empty new env falls through to legacy",
			newEnv:      ptr(""),
			legacyEnv:   ptr("/legacy/env"),
			want:        "/legacy/env",
			wantWarning: true,
		},
		{
			name: "default path when neither env var set",
			want: filepath.Join(home, ".config", "everything-cli"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", home)
			if tc.newEnv != nil {
				t.Setenv(EnvConfigDir, *tc.newEnv)
			}
			if tc.legacyEnv != nil {
				t.Setenv(LegacyEnvConfigDir, *tc.legacyEnv)
			}
			stderrBuf := captureStderr(t)

			got, err := ResolveDir(tc.root)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
			if tc.wantWarning {
				warning := stderrBuf.String()
				assert.Contains(t, warning, LegacyEnvConfigDir)
				assert.Contains(t, warning, EnvConfigDir)
				assert.Contains(t, warning, "deprecated")
			} else {
				assert.Empty(t, stderrBuf.String())
			}
		})
	}
}

func ptr(s string) *string { return &s }

func TestNewStoreUsesResolvedDir(t *testing.T) {
	store, err := NewStore(afero.NewMemMapFs(), "/custom/root")
	require.NoError(t, err)
	assert.Equal(t, "/custom/root", store.Dir())
	assert.Equal(t, "/custom/root/credentials.json", store.CredentialsPath())
	assert.Equal(t, "/custom/root/accounts/google/work.json", store.AccountPath("work"))
}
