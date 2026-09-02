package account

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/auth"
)

// TestGetIdentityShowsMetadataWithoutTokenValues: every output format shows
// the email, scopes and token expiry — and never the access or refresh
// token value, asserted against the actual rendered bytes.
func TestGetIdentityShowsMetadataWithoutTokenValues(t *testing.T) {
	cfg, root, out := newAccountEnv(t, googleSpec)
	seedGoogleAccount(t, cfg, "work", "work@example.com")

	for _, format := range []string{"json", "table", "toon"} {
		t.Run(format, func(t *testing.T) {
			outStr, err := execute(t, root, out, "account", "get", "work", "--format", format)
			require.NoError(t, err)

			assert.Contains(t, outStr, "work@example.com")
			assert.Contains(t, outStr, auth.ScopeUserEmail)
			assert.Contains(t, outStr, "2026-01-02T03:04:05Z", "token expiry must be shown")
			assert.NotContains(t, outStr, "secret-access-work",
				"the access token value must never be printed")
			assert.NotContains(t, outStr, "secret-refresh-work",
				"the refresh token value must never be printed")
			assert.NotContains(t, outStr, "access_token",
				"token fields must not appear by name either")
			assert.NotContains(t, outStr, "refresh_token")
		})
	}
}

// TestGetIdentityJSONKeysAreSnakeCase: the identity variant's JSON view
// carries exactly the metadata keys name, email, scopes, token_expiry.
func TestGetIdentityJSONKeysAreSnakeCase(t *testing.T) {
	cfg, root, out := newAccountEnv(t, googleSpec)
	seedGoogleAccount(t, cfg, "work", "work@example.com")

	outStr, err := execute(t, root, out, "account", "get", "work", "--format", "json")
	require.NoError(t, err)

	var view map[string]any
	require.NoError(t, json.Unmarshal([]byte(outStr), &view))
	keys := make([]string, 0, len(view))
	for k := range view {
		keys = append(keys, k)
	}
	assert.ElementsMatch(t, []string{"name", "email", "scopes", "token_expiry"}, keys)
	assert.Equal(t, "work@example.com", view["email"])
}

// TestGetIdentityTableHeadersUpperCased: go-pretty StyleLight upper-cases
// header cells, so table assertions use upper-case headers.
func TestGetIdentityTableHeadersUpperCased(t *testing.T) {
	cfg, root, out := newAccountEnv(t, googleSpec)
	seedGoogleAccount(t, cfg, "work", "work@example.com")

	outStr, err := execute(t, root, out, "account", "get", "work", "--format", "table")
	require.NoError(t, err)

	assert.Contains(t, outStr, "NAME")
	assert.Contains(t, outStr, "EMAIL")
	assert.Contains(t, outStr, "SCOPES")
	assert.Contains(t, outStr, "TOKEN_EXPIRY")
}

// TestGetPlainShowsMetadataOnly: the key-based variant shows name and
// provider in every format, and the API key never reaches output.
func TestGetPlainShowsMetadataOnly(t *testing.T) {
	cfg, root, out := newAccountEnv(t, linearSpec)
	seedKeyAccount(t, cfg, linearSpec.ProviderID, "work", "test-key-123")

	for _, format := range []string{"json", "table", "toon"} {
		t.Run(format, func(t *testing.T) {
			outStr, err := execute(t, root, out, "account", "get", "work", "--format", format)
			require.NoError(t, err)

			assert.Contains(t, outStr, "work")
			assert.Contains(t, outStr, "linear")
			assert.NotContains(t, outStr, "test-key-123",
				"format %s leaked the API key", format)
		})
	}
}

// TestGetUnknownAccountErrors, for both variants.
func TestGetUnknownAccountErrors(t *testing.T) {
	for name, spec := range map[string]Spec{"identity": googleSpec, "plain": linearSpec} {
		t.Run(name, func(t *testing.T) {
			_, root, out := newAccountEnv(t, spec)

			_, err := execute(t, root, out, "account", "get", "ghost", "--format", "json")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "ghost")
		})
	}
}
