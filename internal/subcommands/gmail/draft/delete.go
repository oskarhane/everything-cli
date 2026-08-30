package draft

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
)

// newDeleteCmd returns `gmail draft delete`: permanently remove a draft.
// Permanent deletion cannot be undone, so it refuses to run without --force.
func newDeleteCmd(_ *app.Config, newSvc serviceFunc) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Permanently delete a Gmail draft (destructive)",
		Example: `# See the refusal without --force
google-cli gmail draft delete draft_19c2a4b7

# Actually delete the draft permanently
google-cli gmail draft delete draft_19c2a4b7 --force`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !force {
				return fmt.Errorf("refusing to permanently delete draft %q without --force (this cannot be undone)", args[0])
			}
			svc, err := newDraftService(cmd.Context(), newSvc)
			if err != nil {
				return err
			}
			return svc.DeleteDraft(cmd.Context(), args[0])
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Permanently delete the draft instead of refusing")
	return cmd
}
