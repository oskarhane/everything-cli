package calendarlist

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/providers/google/calendar/service"
)

// newListCmd returns `calendar list`: every calendar on the account's
// calendar list.
func newListCmd(cfg *app.Config, newSvc service.Dialer[service.CalendarService]) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List calendars",
		Example: `# List calendars as JSON
google-cli calendar list --format json

# List calendars as a table
google-cli calendar list --format table`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			entries, err := svc.ListCalendarList(cmd.Context())
			if err != nil {
				return err
			}
			printCalendarList(cmd, cfg, entries)
			return nil
		},
	}
}
