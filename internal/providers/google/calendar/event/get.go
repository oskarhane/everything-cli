package event

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/google/calendar/service"
)

// newGetCmd returns `calendar event get`: one event by id. The id may be a
// master id (whole series) or an instance id (`<masterId>_<UTC start>`);
// resolve instance ids via `calendar event list` or `event instances`.
func newGetCmd(cfg *app.Config, newSvc service.Dialer[service.EventService]) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <event-id>",
		Short: "Show an event by id (master or instance)",
		Example: `# Show a single event as JSON
everything-cli google calendar event get abc123 --format json

# Show one occurrence of a recurring series (instance ids end in _<UTC time>)
everything-cli google calendar event get kq3abc123_20260929T030000Z --format json

# Show the recurring master with its RRULE lines on another calendar
everything-cli google calendar event get kq3abc123 --calendar work@example.com --format table`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			calendarID, _ := cmd.Flags().GetString("calendar")
			ev, err := svc.GetEvent(cmd.Context(), calendarID, args[0])
			if err != nil {
				return err
			}
			printEventView(cmd, cfg, ev)
			return nil
		},
	}
	cmd.Flags().String("calendar", "primary", "Calendar id")
	return cmd
}
