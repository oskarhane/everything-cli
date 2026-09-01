package label

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/google/gmail/service"
)

// newDeleteCmd returns `gmail label delete`: remove a label. Deleting is
// destructive and removes the label from every message, so it refuses to run
// without --force.
func newDeleteCmd(_ *app.Config, newSvc service.Dialer[service.GmailService]) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a Gmail label (destructive)",
		Example: `# See the refusal without --force
everything-cli gmail label delete Label_42

# Actually delete the label
everything-cli gmail label delete Label_42 --force`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !force {
				return fmt.Errorf("refusing to delete label %q without --force (this removes it from every message)", args[0])
			}
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			return svc.DeleteLabel(cmd.Context(), args[0])
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Delete the label instead of refusing")
	return cmd
}
