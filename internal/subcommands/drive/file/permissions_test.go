package file

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/google-cli/internal/subcommands/cmdtest"
)

func TestPermissionsJSON(t *testing.T) {
	svc := &fakeService{perms: seedPermissions()}
	out := cmdtest.RunCmd(t, newLeafCmd(newPermissionsCmd, svc, "json"), "file_1")

	rows, ok := cmdtest.DecodeJSON(t, out).([]any)
	require.True(t, ok, "expected a JSON array, got: %s", out)
	require.Len(t, rows, 3)
	for _, raw := range rows {
		row := raw.(map[string]any)
		cmdtest.RequireSnakeCase(t, cmdtest.JSONKeys(t, row))
		require.ElementsMatch(t, permissionFields, cmdtest.JSONKeys(t, row))
	}
	first := rows[0].(map[string]any)
	require.Equal(t, "perm_1", first["id"])
	require.Equal(t, "user", first["type"])
	require.Equal(t, "writer", first["role"])
	require.Equal(t, "alice@example.com", first["email_address"])
	require.Equal(t, "Alice", first["display_name"])
	require.Equal(t, false, first["deleted"])
	// One row collapses to a single object per the output convention.
	require.Equal(t, "team@example.com", cmdtest.DecodeJSON(t, out).([]any)[1].(map[string]any)["email_address"])
}

func TestPermissionsTable(t *testing.T) {
	svc := &fakeService{perms: seedPermissions()}
	out := cmdtest.RunCmd(t, newLeafCmd(newPermissionsCmd, svc, "table"), "file_1")

	// go-pretty StyleLight upper-cases headers.
	for _, header := range []string{"ID", "TYPE", "ROLE", "EMAIL_ADDRESS", "DISPLAY_NAME", "DELETED"} {
		require.Contains(t, out, header)
	}
	require.Contains(t, out, "alice@example.com")
	require.Contains(t, out, "anyoneWithLink")
}

func TestPermissionsEmpty(t *testing.T) {
	svc := &fakeService{perms: nil}
	out := cmdtest.RunCmd(t, newLeafCmd(newPermissionsCmd, svc, "json"), "file_1")

	require.Contains(t, out, "[]")
}

func TestPermissionsPropagatesAPIError(t *testing.T) {
	svc := &fakeService{err: errors.New("googleapi: Error 404")}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newPermissionsCmd, svc, "json"), "file_1")

	require.Contains(t, err.Error(), "googleapi: Error 404")
}

func TestPermissionsRequiresExactlyOneArg(t *testing.T) {
	svc := &fakeService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newPermissionsCmd, svc, "json"))

	require.Contains(t, err.Error(), "accepts 1 arg")
}

func TestPermissionsRowShape(t *testing.T) {
	row := permissionRow(seedPermissions()[0])

	cmdtest.RequireSnakeCase(t, keysOf(row))
	require.Equal(t, "perm_1", row["id"])
	require.Equal(t, "user", row["type"])
	require.Equal(t, "writer", row["role"])
	require.Equal(t, "alice@example.com", row["email_address"])
	require.Equal(t, "Alice", row["display_name"])
	require.Equal(t, false, row["deleted"])
}
