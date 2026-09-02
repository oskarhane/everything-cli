package acl

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	calendar "google.golang.org/api/calendar/v3"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestAddReader(t *testing.T) {
	svc := &fakeService{}
	out := cmdtest.RunCmd(t, newLeafCmd(newAddCmd, svc, "json"),
		"primary", "--scope-user", "colleague@example.com", "--role", "reader")

	require.Equal(t, "primary", svc.insertID)
	require.NotNil(t, svc.inserted)
	require.Equal(t, &calendar.AclRule{
		Scope: &calendar.AclRuleScope{Type: "user", Value: "colleague@example.com"},
		Role:  "reader",
	}, svc.inserted)

	// The created rule is echoed as output.
	row, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok)
	keys := cmdtest.JSONKeys(t, row)
	require.ElementsMatch(t, []string{"id", "scope_type", "scope_value", "role"}, keys)
	cmdtest.RequireSnakeCase(t, keys)
	require.Equal(t, "user:colleague@example.com", row["id"])
	require.Equal(t, "user", row["scope_type"])
	require.Equal(t, "reader", row["role"])
}

func TestAddWriter(t *testing.T) {
	svc := &fakeService{}
	cmdtest.RunCmd(t, newLeafCmd(newAddCmd, svc, "json"),
		"primary", "--scope-user", "teammate@example.com", "--role", "writer")

	require.NotNil(t, svc.inserted)
	require.Equal(t, "writer", svc.inserted.Role)
	require.Equal(t, "teammate@example.com", svc.inserted.Scope.Value)
}

func TestAddRejectsInvalidRole(t *testing.T) {
	// The role is validated client-side so a bad value never reaches the
	// API.
	svc := &fakeService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newAddCmd, svc, "json"),
		"primary", "--scope-user", "colleague@example.com", "--role", "owner")

	require.Contains(t, err.Error(), `invalid --role "owner"`)
	require.Contains(t, err.Error(), "reader or writer")
	require.Nil(t, svc.inserted, "invalid input must not reach the API")
}

func TestAddRejectsEmptyRole(t *testing.T) {
	svc := &fakeService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newAddCmd, svc, "json"),
		"primary", "--scope-user", "colleague@example.com")

	require.Contains(t, err.Error(), "invalid --role")
	require.Nil(t, svc.inserted)
}

func TestAddRequiresScopeUser(t *testing.T) {
	svc := &fakeService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newAddCmd, svc, "json"), "primary", "--role", "reader")

	require.Contains(t, err.Error(), "--scope-user is required")
	require.Nil(t, svc.inserted, "missing scope must not reach the API")
}

func TestAddPropagatesAPIError(t *testing.T) {
	svc := &fakeService{insertErr: errors.New("googleapi: Error 400 duplicate acl rule")}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newAddCmd, svc, "json"),
		"primary", "--scope-user", "colleague@example.com", "--role", "reader")

	require.Contains(t, err.Error(), "googleapi: Error 400")
}
