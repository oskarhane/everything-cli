package file

import (
	"testing"

	"github.com/stretchr/testify/require"

	drive "google.golang.org/api/drive/v3"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

// TestShareEchoesGrantedNotRequested asserts the confirmation reflects the
// API RESPONSE (role/type/id/expiry), not the flags: Google may coerce the
// grant, so the echoed line is the only audit trail.
func TestShareEchoesGrantedNotRequested(t *testing.T) {
	svc := &fakeService{
		CoercedGrant: &drive.Permission{
			Id:             "perm_coerced",
			Type:           "anyone",
			Role:           "writer",
			ExpirationTime: "2026-12-31T00:00:00Z",
		},
	}
	out := cmdtest.RunCmd(t, newLeafCmd(newShareCmd, svc, "json"),
		"file_1", "--role", "reader", "--email", "alice@example.com")

	require.Equal(t, "file_1", svc.grantedFileID)
	require.Equal(t, "reader", svc.grantedPerm.Role, "request still carries the flag role")
	require.Contains(t, out, "Granted writer on file_1 to alice@example.com",
		"echo must show the granted role, not the requested role")
	require.NotContains(t, out, "Granted reader")
	require.Contains(t, out, "permission perm_coerced, type anyone, expires 2026-12-31T00:00:00Z")
}

// TestShareEchoOmitsExpiresWhenNoneGranted: the expiry clause appears only
// when the response carries one.
func TestShareEchoOmitsExpiresWhenNoneGranted(t *testing.T) {
	svc := &fakeService{
		CoercedGrant: &drive.Permission{Id: "perm_coerced", Type: "user", Role: "reader", EmailAddress: "alice@example.com"},
	}
	out := cmdtest.RunCmd(t, newLeafCmd(newShareCmd, svc, "json"),
		"file_1", "--role", "reader", "--email", "alice@example.com", "--expires", "2027-01-01T00:00:00Z")

	require.Contains(t, out, "Granted reader on file_1 to alice@example.com (permission perm_coerced, type user)")
	require.NotContains(t, out, "expires")
}

func TestShareUserGrant(t *testing.T) {
	svc := &fakeService{}
	out := cmdtest.RunCmd(t, newLeafCmd(newShareCmd, svc, "json"),
		"file_1", "--role", "reader", "--email", "alice@example.com")

	require.Equal(t, "file_1", svc.grantedFileID)
	require.Equal(t, "user", svc.grantedPerm.Type)
	require.Equal(t, "alice@example.com", svc.grantedPerm.EmailAddress)
	require.Equal(t, "reader", svc.grantedPerm.Role)
	require.Contains(t, out, "Granted reader on file_1 to alice@example.com (permission perm_new, type user)")
}

func TestShareAnyoneGrant(t *testing.T) {
	svc := &fakeService{}
	out := cmdtest.RunCmd(t, newLeafCmd(newShareCmd, svc, "json"),
		"file_1", "--role", "commenter", "--anyone")

	require.Equal(t, "anyone", svc.grantedPerm.Type)
	require.Equal(t, false, svc.grantedPerm.AllowFileDiscovery, "link grant, not discoverable")
	require.Empty(t, svc.grantedPerm.EmailAddress)
	require.Contains(t, out, "Granted commenter on file_1 to anyone (permission perm_new, type anyone)")
}

func TestShareDomainGrant(t *testing.T) {
	svc := &fakeService{}
	out := cmdtest.RunCmd(t, newLeafCmd(newShareCmd, svc, "json"),
		"file_1", "--role", "writer", "--domain", "example.com")

	require.Equal(t, "domain", svc.grantedPerm.Type)
	require.Equal(t, "example.com", svc.grantedPerm.Domain)
	require.Contains(t, out, "Granted writer on file_1 to domain example.com (permission perm_new, type domain)")
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
		{"email no at", []string{"file_1", "--role", "reader", "--email", "alice"},
			`invalid --email "alice"`},
		{"email two at", []string{"file_1", "--role", "reader", "--email", "a@b@example.com"},
			`invalid --email "a@b@example.com"`},
		{"email empty local part", []string{"file_1", "--role", "reader", "--email", "@example.com"},
			`invalid --email "@example.com"`},
		{"email spaces", []string{"file_1", "--role", "reader", "--email", "alice smi th@example.com"},
			`invalid --email "alice smi th@example.com"`},
		{"domain spaces", []string{"file_1", "--role", "reader", "--domain", "exa mple.com"},
			`invalid --domain "exa mple.com"`},
		{"domain contains at", []string{"file_1", "--role", "reader", "--domain", "user@example.com"},
			`invalid --domain "user@example.com"`},
		{"domain no dot", []string{"file_1", "--role", "reader", "--domain", "example"},
			`invalid --domain "example"`},
		{"domain empty", []string{"file_1", "--role", "reader", "--domain", ""},
			"exactly one of --email, --anyone, or --domain is required (got 0)"},
		{"domain leading dot", []string{"file_1", "--role", "reader", "--domain", ".example.com"},
			`invalid --domain ".example.com"`},
		{"domain trailing hyphen label", []string{"file_1", "--role", "reader", "--domain", "ex-.com"},
			`invalid --domain "ex-.com"`},
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

func TestValidateEmail(t *testing.T) {
	for _, tt := range []struct {
		in   string
		want string // non-empty = expected failure reason
	}{
		{in: "alice@example.com", want: ""},
		{in: "a.b+c@sub.example.co.uk", want: ""},
		{in: "", want: "must not be empty"},
		{in: "   ", want: "must not be empty"},
		{in: "alice", want: "missing '@'"},
		{in: "@example.com", want: "invalid"},
		{in: `Alice <alice@example.com>`, want: "bare email address"},
		{in: `alice@example.com (work)`, want: "bare email address"},
	} {
		got := validateEmail(tt.in)
		if tt.want == "" {
			require.Empty(t, got, "input %q should be valid", tt.in)
		} else {
			require.Contains(t, got, tt.want, "input %q", tt.in)
		}
	}
}

func TestValidateDomain(t *testing.T) {
	for _, tt := range []struct {
		in   string
		want string // non-empty = expected failure reason substring
	}{
		{in: "example.com", want: ""},
		{in: "sub.example.co.uk", want: ""},
		{in: "", want: "must not be empty"},
		{in: "   ", want: "must not be empty"},
		{in: "exa mple.com", want: "spaces"},
		{in: "user@example.com", want: "@\""},
		{in: "example", want: "dot"},
		{in: ".example.com", want: "dot"},
		{in: "example.com.", want: "dot"},
		{in: "exa..com", want: "empty labels"},
		{in: "ex-.com", want: "hyphen"},
		{in: "-ex.com", want: "hyphen"},
	} {
		got := validateDomain(tt.in)
		if tt.want == "" {
			require.Empty(t, got, "input %q should be valid", tt.in)
		} else {
			require.Contains(t, got, tt.want, "input %q", tt.in)
		}
	}
}
