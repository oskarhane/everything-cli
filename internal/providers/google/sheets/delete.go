package sheets

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
)

// newDeleteCmd returns `sheets delete`: permanently remove the spreadsheet's
// underlying Drive file, bypassing the trash. Permanent deletion cannot be
// undone, so it refuses to run without --force; trash is the recoverable
// alternative.
func newDeleteCmd(_ *app.Config, newSvc service.Dialer[service.FileService]) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "delete <spreadsheet-id>",
		Short: "Permanently delete a spreadsheet (destructive)",
		Example: `# See the refusal without --force
everything-cli google sheets delete 1AbCdEfGh

# Actually delete the spreadsheet permanently
everything-cli google sheets delete 1AbCdEfGh --force`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !force {
				return fmt.Errorf("refusing to permanently delete spreadsheet %q without --force (this cannot be undone; use \"everything-cli google drive file trash <id>\" instead)", args[0])
			}
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			return svc.DeleteFile(cmd.Context(), args[0])
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Permanently delete the spreadsheet instead of refusing")
	return cmd
}
