package slides

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/subcommands/drive/service"
)

// newDeleteCmd returns `slides delete`: permanently remove the presentation
// file from Drive. The file is gone, not trashed, so --force is required and
// the refusal names what would be deleted.
func newDeleteCmd(_ *app.Config, newSvc service.Dialer[service.FileService]) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "delete <presentation-id>",
		Short: "Permanently delete a presentation (destructive)",
		Example: `# See the refusal without --force
google-cli slides delete 1AbCpresentationID

# Actually delete the presentation permanently
google-cli slides delete 1AbCpresentationID --force`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !force {
				return fmt.Errorf("refusing to permanently delete presentation %q without --force (this cannot be undone; use \"google-cli drive file trash <id>\" instead)", args[0])
			}
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			return svc.DeleteFile(cmd.Context(), args[0])
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Permanently delete the presentation instead of refusing")
	return cmd
}
