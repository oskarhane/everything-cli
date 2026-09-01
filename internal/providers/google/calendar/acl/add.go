package acl

import (
	"fmt"

	"github.com/spf13/cobra"

	calendar "google.golang.org/api/calendar/v3"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/subcommands/calendar/service"
)

// newAddCmd returns `calendar acl add`: share a calendar with one user. The
// role is validated client-side so a bad value never reaches the API.
func newAddCmd(cfg *app.Config, newSvc service.Dialer[service.CalendarService]) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <calendar-id>",
		Short: "Share a calendar with a user",
		Example: `# Share the primary calendar as read-only
google-cli calendar acl add primary --scope-user colleague@example.com --role reader

# Share a calendar with write access
google-cli calendar acl add primary --scope-user teammate@example.com --role writer`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f := cmd.Flags()
			scopeUser, _ := f.GetString("scope-user")
			if scopeUser == "" {
				return fmt.Errorf("--scope-user is required: the email of the user to share with")
			}
			role, _ := f.GetString("role")
			if role != "reader" && role != "writer" {
				return fmt.Errorf("invalid --role %q: expected reader or writer", role)
			}
			rule := &calendar.AclRule{
				Scope: &calendar.AclRuleScope{Type: "user", Value: scopeUser},
				Role:  role,
			}
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			inserted, err := svc.InsertAcl(cmd.Context(), args[0], rule)
			if err != nil {
				return err
			}
			printAclRule(cmd, cfg, aclRow(inserted))
			return nil
		},
	}
	cmd.Flags().String("scope-user", "", "Email of the user to share with")
	cmd.Flags().String("role", "", "Access role: reader or writer")
	return cmd
}
