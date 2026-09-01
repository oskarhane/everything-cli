package file

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/google-cli/internal/subcommands/cmdtest"
)

func TestShareUserGrant(t *testing.T) {
	svc := &fakeService{}
	out := cmdtest.RunCmd(t, newLeafCmd(newShareCmd, svc, "json"),
		"file_1", "--role", "reader", "--email", "alice@example.com")

	require.Equal(t, "file_1", svc.grantedFileID)
	require.Equal(t, "user", svc.grantedPerm.Type)
	require.Equal(t, "alice@example.com", svc.grantedPerm.EmailAddress)
	require.Equal(t, "reader", svc.grantedPerm.Role)
	require.Contains(t, out, "Granted reader on file_1 to alice@example.com")
}

func TestShareAnyoneGrant(t *testing.T) {
	svc := &fakeService{}
	out := cmdtest.RunCmd(t, newLeafCmd(newShareCmd, svc, "json"),
		"file_1", "--role", "commenter", "--anyone")

	require.Equal(t, "anyone", svc.grantedPerm.Type)
	require.Equal(t, false, svc.grantedPerm.AllowFileDiscovery, "link grant, not discoverable")
	require.Empty(t, svc.grantedPerm.EmailAddress)
	require.Contains(t, out, "Granted commenter on file_1 to anyone")
}

func TestShareDomainGrant(t *testing.T) {
	svc := &fakeService{}
	out := cmdtest.RunCmd(t, newLeafCmd(newShareCmd, svc, "json"),
		"file_1", "--role", "writer", "--domain", "example.com")

	require.Equal(t, "domain", svc.grantedPerm.Type)
	require.Equal(t, "example.com", svc.grantedPerm.Domain)
	require.Contains(t, out, "Granted writer on file_1 to domain example.com")
}

func TestShareExpiresPassthrough(t *testing.T) {
	svc := &fakeService{}
	cmdtest.RunCmd(t, newLeafCmd(newShareCmd, svc, "json"),
		"file_1", "--role", "reader", "--email", "alice@example.com", "--expires", "2027-01-01T00:00:00Z")

	require.Equal(t, "2027-01-01T00:00:00Z", svc.grantedPerm.ExpirationTime)
}

func TestShareValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"missing role", []string{"file_1", "--email", "a@example.com"},
			`required flag(s) "role" not set`},
		{"bad role", []string{"file_1", "--role", "admin", "--email", "a@example.com"},
			`invalid --role "admin": role must be reader, commenter, or writer`},
		{"no target", []string{"file_1", "--role", "reader"},
			"exactly one of --email, --anyone, or --domain is required (got 0)"},
		{"two targets", []string{"file_1", "--role", "reader", "--email", "a@example.com", "--anyone"},
			"exactly one of --email, --anyone, or --domain is required (got 2)"},
		{"all targets", []string{"file_1", "--role", "reader", "--email", "a@example.com", "--anyone", "--domain", "example.com"},
			"exactly one of --email, --anyone, or --domain is required (got 3)"},
		{"bad expires", []string{"file_1", "--role", "reader", "--email", "a@example.com", "--expires", "next tuesday"},
			`invalid --expires "next tuesday": must be an RFC 3339 timestamp`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeService{}
			_, err := cmdtest.RunCmdErr(t, newLeafCmd(newShareCmd, svc, "json"), tt.args...)

			require.Contains(t, err.Error(), tt.want)
			require.Empty(t, svc.grantedFileID, "no grant may reach the service on validation failure")
		})
	}
}

func TestShareValidExpiresNotRejected(t *testing.T) {
	svc := &fakeService{}
	cmdtest.RunCmd(t, newLeafCmd(newShareCmd, svc, "json"),
		"file_1", "--role", "reader", "--email", "a@example.com", "--expires", "2027-01-01T00:00:00Z")

	require.Equal(t, "file_1", svc.grantedFileID)
}
