package event

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/providers/google/calendar/service"
)

// newMoveCmd returns `calendar event move`: move an event between
// calendars. Works for master and instance ids; moving an instance moves
// only that occurrence.
func newMoveCmd(cfg *app.Config, newSvc service.Dialer[service.EventService]) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "move <event-id>",
		Short: "Move an event to another calendar",
		Example: `# Move an event to a secondary calendar
google-cli calendar event move abc123 --to-calendar work.group.calendar.google.com

# Move one occurrence of a recurring series
google-cli calendar event move kq3abc123_20260929T030000Z --to-calendar work.group.calendar.google.com

# Move an event off a secondary calendar back to the primary one
google-cli calendar event move abc123 --calendar work.group.calendar.google.com --to-calendar primary`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f := cmd.Flags()
			dest := flagString(f, "to-calendar")
			if dest == "" {
				return fmt.Errorf("--to-calendar is required: the destination calendar id")
			}
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			moved, err := svc.MoveEvent(cmd.Context(), flagString(f, "calendar"), args[0], dest)
			if err != nil {
				return err
			}
			printEventView(cmd, cfg, moved)
			return nil
		},
	}
	f := cmd.Flags()
	f.String("to-calendar", "", "Destination calendar id (required)")
	f.String("calendar", "primary", "Source calendar id")
	return cmd
}
