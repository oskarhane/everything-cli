package message

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/subcommands/gmail/service"
)

// newDeleteCmd returns `gmail message delete`: permanently remove a message.
// Permanent deletion cannot be undone, so it refuses to run without --force;
// trash is the recoverable alternative.
func newDeleteCmd(_ *app.Config, newSvc service.Dialer[service.MessageService]) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Permanently delete a Gmail message (destructive)",
		Example: `# See the refusal without --force
google-cli gmail message delete 19c2a4b7

# Actually delete the message permanently
google-cli gmail message delete 19c2a4b7 --force`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !force {
				return fmt.Errorf("refusing to permanently delete message %q without --force (this cannot be undone; use trash instead)", args[0])
			}
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			return svc.DeleteMessage(cmd.Context(), args[0])
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Permanently delete the message instead of refusing")
	return cmd
}
