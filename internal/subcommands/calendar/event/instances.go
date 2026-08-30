package event

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/subcommands/calendar/service"
)

// newInstancesCmd returns `calendar event instances`: expand one recurring
// event into its occurrences. The argument is the master (recurring) event
// id; the response carries the instance ids that per-occurrence commands
// (`update --this-only`, `delete --this-only`, `decline`) accept.
func newInstancesCmd(cfg *app.Config, newSvc service.Dialer[service.EventService]) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "instances <master-id>",
		Short: "List the occurrences of a recurring event",
		Example: `# Expand a weekly series into its occurrences as JSON
google-cli calendar event instances kq3abc123 --format json

# Only the next two weeks of occurrences
google-cli calendar event instances kq3abc123 --from now --to +14d --format table

# Occurrences of a series on another calendar
google-cli calendar event instances kq3abc123 --calendar work@example.com --format json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			f := cmd.Flags()
			calendarID, _ := f.GetString("calendar")
			fromRaw, _ := f.GetString("from")
			toRaw, _ := f.GetString("to")
			now := nowFunc()
			from, err := parseWindowTime(fromRaw, now)
			if err != nil {
				return err
			}
			to, err := parseWindowTime(toRaw, now)
			if err != nil {
				return err
			}
			events, err := svc.ListInstances(cmd.Context(), service.ListInstancesParams{
				CalendarID: calendarID,
				EventID:    args[0],
				TimeMin:    from,
				TimeMax:    to,
			})
			if err != nil {
				return err
			}
			printEventList(cmd, cfg, events)
			return nil
		},
	}
	f := cmd.Flags()
	f.String("calendar", "primary", "Calendar id")
	f.String("from", "", "Window start: RFC3339, date, or relative (now, -1d, +7d); empty = unbounded")
	f.String("to", "", "Window end: RFC3339, date, or relative; empty = unbounded")
	return cmd
}
