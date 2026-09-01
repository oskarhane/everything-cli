package docs

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/subcommands/drive/service"
)

// newDeleteCmd returns `docs delete`: permanently remove the document's
// Drive file, bypassing the trash. Permanent deletion cannot be undone, so
// it refuses to run without --force; drive file trash is the recoverable
// alternative.
func newDeleteCmd(_ *app.Config, newSvc service.Dialer[service.FileService]) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "delete <doc-id>",
		Short: "Permanently delete a Google Doc (destructive)",
		Example: `# See the refusal without --force
google-cli docs delete 1AbCdEfGh

# Actually delete the document permanently
google-cli docs delete 1AbCdEfGh --force`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !force {
				return fmt.Errorf("refusing to permanently delete document %q without --force (this cannot be undone; use \"google-cli drive file trash <id>\" instead)", args[0])
			}
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			return svc.DeleteFile(cmd.Context(), args[0])
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Permanently delete the document instead of refusing")
	return cmd
}
