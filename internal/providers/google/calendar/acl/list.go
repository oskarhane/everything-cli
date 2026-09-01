package acl

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/providers/google/calendar/service"
)

// newListCmd returns `calendar acl list`: the sharing rules of one calendar.
func newListCmd(cfg *app.Config, newSvc service.Dialer[service.CalendarService]) *cobra.Command {
	return &cobra.Command{
		Use:   "list <calendar-id>",
		Short: "List a calendar's sharing rules",
		Example: `# List the primary calendar's sharing rules as JSON
google-cli calendar acl list primary --format json

# List sharing rules as a table
google-cli calendar acl list primary --format table`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			rules, err := svc.ListAcl(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			printAclRules(cmd, cfg, rules)
			return nil
		},
	}
}
