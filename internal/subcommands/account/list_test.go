package account

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestListAggregatesProvidersJSON: JSON output covers accounts of every
// provider with snake_case keys and marks each provider's default with
// "default": true — the same field name the per-provider lists use.
func TestListAggregatesProvidersJSON(t *testing.T) {
	cfg, root, out := newAccountEnv(t)
	seedAccount(t, cfg, "google", "personal", "me@example.com")
	seedAccount(t, cfg, "google", "work", "work@example.com")
	seedAccount(t, cfg, "linear", "work", "me@linear.example.com")
	require.NoError(t, newStore(t, cfg).SetDefaultAccountFor("google", "work"))

	outStr, err := execute(t, root, out, "account", "list", "--format", "json")
	require.NoError(t, err)

	var rows []map[string]any
	require.NoError(t, json.Unmarshal([]byte(outStr), &rows))
	require.Len(t, rows, 3)

	type key struct{ provider, name string }
	byKey := map[key]map[string]any{}
	for _, row := range rows {
		keys := make([]string, 0, len(row))
		for k := range row {
			keys = append(keys, k)
		}
		assert.ElementsMatch(t, []string{"name", "provider", "identity", "default"}, keys,
			"JSON keys must be snake_case: name, provider, identity, default")
		byKey[key{row["provider"].(string), row["name"].(string)}] = row
	}
	assert.Equal(t, true, byKey[key{"google", "work"}]["default"],
		"the google default account must be marked")
	assert.Equal(t, false, byKey[key{"google", "personal"}]["default"])
	assert.Equal(t, "work@example.com", byKey[key{"google", "work"}]["identity"])
	assert.Equal(t, true, byKey[key{"linear", "work"}]["default"],
		"the first account of a provider is its default")
	assert.Equal(t, "email=me@linear.example.com", byKey[key{"linear", "work"}]["identity"],
		"non-Google accounts render their identity map")
}

// TestListTable: the table prints upper-case headers (go-pretty StyleLight)
// and marks each provider's default row with the same "(default)" marker
// the per-provider lists render.
func TestListTable(t *testing.T) {
	cfg, root, out := newAccountEnv(t)
	seedAccount(t, cfg, "google", "work", "work@example.com")

	outStr, err := execute(t, root, out, "account", "list", "--format", "table")
	require.NoError(t, err)

	assert.Contains(t, outStr, "NAME", "headers are upper-cased")
	assert.Contains(t, outStr, "PROVIDER")
	assert.Contains(t, outStr, "IDENTITY")
	assert.Contains(t, outStr, "DEFAULT")
	assert.Contains(t, outStr, "work@example.com")
	assert.Contains(t, outStr, "(default)", "the default account's row must be marked like the per-provider lists")
}

// TestListEmptyStore: a store with no accounts lists nothing without error.
func TestListEmptyStore(t *testing.T) {
	_, root, out := newAccountEnv(t)

	outStr, err := execute(t, root, out, "account", "list", "--format", "json")
	require.NoError(t, err)
	assert.Equal(t, "[]\n", outStr)
}

// TestAccountIsReadOnly: management verbs are provider-scoped, so the
// top-level account command must not expose them.
func TestAccountIsReadOnly(t *testing.T) {
	_, root, _ := newAccountEnv(t)

	accountCmd, _, err := root.Find([]string{"account"})
	require.NoError(t, err)
	for _, verb := range []string{"add", "use", "get", "remove"} {
		for _, sub := range accountCmd.Commands() {
			assert.NotEqual(t, verb, sub.Name(),
				"top-level account must not expose %q — it lives under each provider", verb)
		}
	}
}
