package account

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestListMarksDefaultJSON: JSON output covers every account with snake_case
// keys and marks exactly the default account with "default": true.
func TestListMarksDefaultJSON(t *testing.T) {
	cfg, root, out := newAccountEnv(t)
	seedAccount(t, cfg, "personal", "me@example.com")
	seedAccount(t, cfg, "work", "work@example.com")
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

// TestListMarksDefaultTable: the table marks the default account's row with
// an upper-case headers table (go-pretty StyleLight upper-cases headers).
func TestListMarksDefaultTable(t *testing.T) {
	cfg, root, out := newAccountEnv(t)
	seedAccount(t, cfg, "personal", "me@example.com")
	seedAccount(t, cfg, "work", "work@example.com")
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

// TestListEmptyStore: a store with no accounts lists nothing without error.
func TestListEmptyStore(t *testing.T) {
	_, root, out := newAccountEnv(t)

	outStr, err := execute(t, root, out, "account", "list", "--format", "json")
	require.NoError(t, err)
	assert.Equal(t, "[]\n", outStr)
}
