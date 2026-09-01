package file

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/subcommands/drive/service"
)

// newTrashCmd returns `drive file trash`: move a file to Drive's trash.
// Trashing is recoverable (see untrash), unlike delete.
func newTrashCmd(_ *app.Config, newSvc service.Dialer[service.FileService]) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trash <file-id>",
		Short: "Move a Drive file to trash",
		Example: `# Trash a file
google-cli drive file trash 1AbCdEfGh

# Trash a file on another account
google-cli drive file trash 1AbCdEfGh --account work`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			if _, err := svc.TrashFile(cmd.Context(), args[0]); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Trashed file %s\n", args[0]); err != nil {
				return err
			}
			return nil
		},
	}
	return cmd
}
