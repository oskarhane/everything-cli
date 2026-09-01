package event

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/google/calendar/service"
)

// newDeleteCmd returns `calendar event delete`. Deleting an instance id
// cancels only that occurrence (the series keeps running); deleting a master
// id deletes the entire series including every exception. Both are
// destructive, so --force is required and the refusal names the scope.
// --this-only (default) targets the given id; --this-only=false with an
// instance id deletes its master instead, i.e. the whole series.
func newDeleteCmd(_ *app.Config, newSvc service.Dialer[service.EventService]) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <event-id>",
		Short: "Delete an event: one occurrence or a whole series",
		Example: `# See the refusal naming what would be deleted
everything-cli calendar event delete kq3abc123_20260929T030000Z

# Cancel a single occurrence of a recurring series
everything-cli calendar event delete kq3abc123_20260929T030000Z --force

# Delete the entire recurring series
everything-cli calendar event delete kq3abc123 --force`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f := cmd.Flags()
			force, _ := f.GetBool("force")
			thisOnly, _ := f.GetBool("this-only")
			target := args[0]
			if !thisOnly {
				target = masterID(nil, args[0])
			}
			scope := "this deletes the entire series, including every exception"
			if isInstanceID(target) {
				scope = "this cancels 1 occurrence; the series keeps running"
			}
			if !force {
				return fmt.Errorf("refusing to delete event %q without --force (%s)", target, scope)
			}
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			return svc.DeleteEvent(cmd.Context(), flagString(f, "calendar"), target, "all")
		},
	}
	f := cmd.Flags()
	f.String("calendar", "primary", "Calendar id")
	f.Bool("force", false, "Delete instead of refusing")
	f.Bool("this-only", true, "Delete the given id only; false with an instance id deletes its master (the whole series)")
	return cmd
}
