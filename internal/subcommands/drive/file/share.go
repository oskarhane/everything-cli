package file

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	drive "google.golang.org/api/drive/v3"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/subcommands/drive/service"
)

// newShareCmd returns `drive file share`: grant a permission on a file to a
// user (--email), everyone with the link (--anyone), or a whole domain
// (--domain). Exactly one target; --expires sets an expiry, which the Drive
// API only honors on user (and group) permissions.
func newShareCmd(_ *app.Config, newSvc service.Dialer[service.FileService]) *cobra.Command {
	var (
		role    string
		email   string
		anyone  bool
		domain  string
		expires string
	)
	cmd := &cobra.Command{
		Use:   "share <file-id>",
		Short: "Share a Drive file with a user, anyone with the link, or a domain",
		Example: `# Give one user reader access
google-cli drive file share 1AbCdEfGh --role reader --email alice@example.com

# Give one user read access for a year (--expires: user and group permissions only)
google-cli drive file share 1AbCdEfGh --role reader --email alice@example.com --expires 2027-09-01T00:00:00Z

# Let anyone with the link comment
google-cli drive file share 1AbCdEfGh --role commenter --anyone

# Give a whole domain write access
google-cli drive file share 1AbCdEfGh --role writer --domain example.com`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !validRole(role) {
				return fmt.Errorf("invalid --role %q: role must be reader, commenter, or writer", role)
			}
			targets := shareTargets(email, anyone, domain)
			if targets != 1 {
				return fmt.Errorf("exactly one of --email, --anyone, or --domain is required (got %d)", targets)
			}
			if expires != "" {
				if _, err := time.Parse(time.RFC3339, expires); err != nil {
					return fmt.Errorf("invalid --expires %q: must be an RFC 3339 timestamp (e.g. 2027-01-01T00:00:00Z)", expires)
				}
			}
			perm := &drive.Permission{Role: role, ExpirationTime: expires}
			switch {
			case email != "":
				perm.Type, perm.EmailAddress = "user", email
			case domain != "":
				perm.Type, perm.Domain = "domain", domain
			default:
				perm.Type, perm.AllowFileDiscovery = "anyone", false
			}
			permSvc, err := service.As[service.PermissionService](newSvc(cmd.Context()))
			if err != nil {
				return err
			}
			if _, err := permSvc.GrantPermission(cmd.Context(), args[0], perm); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Granted %s on %s to %s\n", role, args[0], shareTarget(email, domain)); err != nil {
				return err
			}
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&role, "role", "", "Access to grant: reader, commenter, or writer")
	f.StringVar(&email, "email", "", "Email address of the user to grant access to")
	f.BoolVar(&anyone, "anyone", false, "Grant anyone with the link the role")
	f.StringVar(&domain, "domain", "", "Domain to grant access to (e.g. example.com)")
	f.StringVar(&expires, "expires", "", "RFC 3339 expiry for the access (user and group permissions only, per the API)")
	_ = cmd.MarkFlagRequired("role")
	return cmd
}

// validRole reports whether role is one the share leaf grants.
func validRole(role string) bool {
	switch role {
	case "reader", "commenter", "writer":
		return true
	}
	return false
}

// shareTargets counts how many mutually exclusive targets the flags set.
func shareTargets(email string, anyone bool, domain string) int {
	n := 0
	if email != "" {
		n++
	}
	if anyone {
		n++
	}
	if domain != "" {
		n++
	}
	return n
}

// shareTarget renders the confirmation's "to" value: the email, the domain,
// or "anyone" for a link grant.
func shareTarget(email, domain string) string {
	switch {
	case email != "":
		return email
	case domain != "":
		return "domain " + domain
	default:
		return "anyone"
	}
}
