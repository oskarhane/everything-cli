package acl

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestListJSON(t *testing.T) {
	svc := &fakeService{rules: seedRules()}
	out := runCmd(t, newLeafCmd(newListCmd, svc, "json"), "primary")

	rows, ok := decodeJSON(t, out).([]any)
	require.True(t, ok, "expected a JSON array, got: %s", out)
	require.Len(t, rows, 2)

	first, ok := rows[0].(map[string]any)
	require.True(t, ok)
	keys := jsonKeys(t, first)
	require.ElementsMatch(t, []string{"id", "scope_type", "scope_value", "role"}, keys)
	requireSnakeCase(t, keys)
	require.Equal(t, "user:colleague@example.com", first["id"])
	require.Equal(t, "user", first["scope_type"])
	require.Equal(t, "colleague@example.com", first["scope_value"])
	require.Equal(t, "reader", first["role"])
}

func TestListJSONScopeWithoutValue(t *testing.T) {
	// The public "default" scope has no value; the key must still appear.
	svc := &fakeService{rules: seedRules()}
	out := runCmd(t, newLeafCmd(newListCmd, svc, "json"), "primary")

	rows := decodeJSON(t, out).([]any)
	second := rows[1].(map[string]any)
	keys := jsonKeys(t, second)
	require.ElementsMatch(t, []string{"id", "scope_type", "scope_value", "role"}, keys)
	require.Equal(t, "default", second["scope_type"])
	require.EqualValues(t, "", second["scope_value"])
}

func TestListTable(t *testing.T) {
	svc := &fakeService{rules: seedRules()}
	out := runCmd(t, newLeafCmd(newListCmd, svc, "table"), "primary")

	// go-pretty StyleLight upper-cases headers.
	for _, header := range []string{"ID", "SCOPE_TYPE", "SCOPE_VALUE", "ROLE"} {
		require.Contains(t, out, header)
	}
	require.Contains(t, out, "colleague@example.com")
	require.Contains(t, out, "reader")
}

func TestListEmpty(t *testing.T) {
	svc := &fakeService{}
	out := runCmd(t, newLeafCmd(newListCmd, svc, "json"), "primary")

	require.Equal(t, []any{}, decodeJSON(t, out))
}

func TestListPropagatesAPIError(t *testing.T) {
	svc := &fakeService{listErr: errors.New("googleapi: Error 404")}
	_, err := runCmdErr(t, newLeafCmd(newListCmd, svc, "json"), "primary")

	require.Contains(t, err.Error(), "googleapi: Error 404")
}

func TestListRequiresExactlyOneArg(t *testing.T) {
	svc := &fakeService{rules: seedRules()}
	_, err := runCmdErr(t, newLeafCmd(newListCmd, svc, "json"))

	require.Contains(t, err.Error(), "accepts 1 arg")
}
