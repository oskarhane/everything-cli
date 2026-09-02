package account

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestListIdentityMarksDefaultJSON: the identity variant's JSON output
// covers every account with snake_case keys name/email/default and marks
// exactly the default account.
func TestListIdentityMarksDefaultJSON(t *testing.T) {
	cfg, root, out := newAccountEnv(t, googleSpec)
	seedGoogleAccount(t, cfg, "personal", "me@example.com")
	seedGoogleAccount(t, cfg, "work", "work@example.com")
	require.NoError(t, newStore(t, cfg).SetDefaultAccount("work"))

	outStr, err := execute(t, root, out, "account", "list", "--format", "json")
	require.NoError(t, err)

	var rows []map[string]any
	require.NoError(t, json.Unmarshal([]byte(outStr), &rows))
	require.Len(t, rows, 2)

	byName := map[string]map[string]any{}
	for _, row := range rows {
		keys := make([]string, 0, len(row))
		for k := range row {
			keys = append(keys, k)
		}
		assert.ElementsMatch(t, []string{"name", "email", "default"}, keys,
			"JSON keys must be snake_case: name, email, default")
		byName[row["name"].(string)] = row
	}
	assert.Equal(t, true, byName["work"]["default"], "the default account must be marked")
	assert.Equal(t, false, byName["personal"]["default"])
	assert.Equal(t, "work@example.com", byName["work"]["email"])
}

// TestListIdentityMarksDefaultTable: the identity variant's table marks the
// default account's row and upper-cases headers (go-pretty StyleLight).
func TestListIdentityMarksDefaultTable(t *testing.T) {
	cfg, root, out := newAccountEnv(t, googleSpec)
	seedGoogleAccount(t, cfg, "personal", "me@example.com")
	seedGoogleAccount(t, cfg, "work", "work@example.com")
	require.NoError(t, newStore(t, cfg).SetDefaultAccount("work"))

	outStr, err := execute(t, root, out, "account", "list", "--format", "table")
	require.NoError(t, err)

	assert.Contains(t, outStr, "(default)", "the default account's row must be marked")
	assert.Contains(t, outStr, "work@example.com")
	assert.Contains(t, outStr, "me@example.com")
	assert.Contains(t, outStr, "NAME", "headers are upper-cased")
	assert.Contains(t, outStr, "EMAIL")
	assert.Contains(t, outStr, "DEFAULT")
}

// TestListPlainMarksDefaultJSON: the key-based variant carries exactly
// name/default, never an email key, and the API key never reaches output.
func TestListPlainMarksDefaultJSON(t *testing.T) {
	cfg, root, out := newAccountEnv(t, linearSpec)
	seedKeyAccount(t, cfg, linearSpec.ProviderID, "work", "test-key-123")
	seedKeyAccount(t, cfg, linearSpec.ProviderID, "personal", "test-key-456")

	outStr, err := execute(t, root, out, "account", "list", "--format", "json")
	require.NoError(t, err)

	var rows []map[string]any
	require.NoError(t, json.Unmarshal([]byte(outStr), &rows))
	require.Len(t, rows, 2)
	for _, row := range rows {
		keys := make([]string, 0, len(row))
		for k := range row {
			keys = append(keys, k)
		}
		assert.ElementsMatch(t, []string{"name", "default"}, keys,
			"the plain variant must not gain an identity column")
	}
	assert.Equal(t, true, rows[1]["default"], "the first added account is the default")
	assert.NotContains(t, outStr, "test-key-123")
	assert.NotContains(t, outStr, "test-key-456")
}

// TestListEmptyStore: a store with no accounts lists nothing without error,
// for both variants.
func TestListEmptyStore(t *testing.T) {
	for name, spec := range map[string]Spec{"identity": googleSpec, "plain": linearSpec} {
		t.Run(name, func(t *testing.T) {
			_, root, out := newAccountEnv(t, spec)

			outStr, err := execute(t, root, out, "account", "list", "--format", "json")
			require.NoError(t, err)
			assert.Equal(t, "[]\n", outStr)
		})
	}
}
