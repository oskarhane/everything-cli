package file

import (
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/spf13/cobra"

	drive "google.golang.org/api/drive/v3"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
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
everything-cli google drive file share 1AbCdEfGh --role reader --email alice@example.com

# Give one user read access for a year (--expires: user and group permissions only)
everything-cli google drive file share 1AbCdEfGh --role reader --email alice@example.com --expires 2027-09-01T00:00:00Z

# Let anyone with the link comment
everything-cli google drive file share 1AbCdEfGh --role commenter --anyone

# Give a whole domain write access
everything-cli google drive file share 1AbCdEfGh --role writer --domain example.com`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !validRole(role) {
				return fmt.Errorf("invalid --role %q: role must be reader, commenter, or writer", role)
			}
			targets := shareTargets(email, anyone, domain)
			if targets != 1 {
				return fmt.Errorf("exactly one of --email, --anyone, or --domain is required (got %d)", targets)
			}
			if email != "" {
				if reason := validateEmail(email); reason != "" {
					return fmt.Errorf("invalid --email %q: %s", email, reason)
				}
			}
			if domain != "" {
				if reason := validateDomain(domain); reason != "" {
					return fmt.Errorf("invalid --domain %q: %s", domain, reason)
				}
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
			granted, err := permSvc.GrantPermission(cmd.Context(), args[0], perm)
			if err != nil {
				return err
			}
			if granted == nil {
				return fmt.Errorf("granting permission on file %s: API returned no permission", args[0])
			}
			// Echo what the API GRANTED, not what was requested: Google may
			// coerce the role or expiry, so the confirmation is the audit
			// trail for an agent-driven CLI.
			details := fmt.Sprintf("permission %s, type %s", granted.Id, granted.Type)
			if granted.ExpirationTime != "" {
				details += ", expires " + granted.ExpirationTime
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Granted %s on %s to %s (%s)\n",
				granted.Role, args[0], shareTarget(email, domain), details); err != nil {
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

// validateEmail sanity-checks --email before dialing the API. It returns ""
// for a valid address, or a short reason for an invalid one.
func validateEmail(email string) string {
	if strings.TrimSpace(email) == "" {
		return "must not be empty"
	}
	addr, err := mail.ParseAddress(email)
	if err != nil {
		return err.Error()
	}
	// Reject odd forms like "Name <a@b.c>" or commented addresses: the value
	// sent to the API must be the bare address the user typed.
	if addr.Address != email {
		return "must be a bare email address (no name or comments)"
	}
	return ""
}

// validateDomain sanity-checks --domain before dialing the API. It is a
// simple hostname check, not full RFC 1123; it exists to catch clearly-broken
// values before an API call. Returns "" for valid, or a short reason.
func validateDomain(domain string) string {
	if strings.TrimSpace(domain) == "" {
		return "must not be empty"
	}
	if strings.ContainsAny(domain, " \t\n") {
		return "must not contain spaces"
	}
	if strings.Contains(domain, "@") {
		return "must not contain \"@\" (use --email for a user grant)"
	}
	if !strings.Contains(domain, ".") {
		return "must contain at least one dot (e.g. example.com)"
	}
	if strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") {
		return "must not start or end with a dot"
	}
	for _, label := range strings.Split(domain, ".") {
		if label == "" {
			return "must not have empty labels (check leading/trailing dots)"
		}
		if strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return fmt.Sprintf("label %q must not start or end with a hyphen", label)
		}
	}
	return ""
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
