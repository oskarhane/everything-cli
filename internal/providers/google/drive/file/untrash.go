package file

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/providers/google/drive/service"
)

// newUntrashCmd returns `drive file untrash`: restore a trashed file.
func newUntrashCmd(_ *app.Config, newSvc service.Dialer[service.FileService]) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "untrash <file-id>",
		Short: "Restore a Drive file from trash",
		Example: `# Restore a trashed file
google-cli drive file untrash 1AbCdEfGh

# Restore a trashed file on another account
google-cli drive file untrash 1AbCdEfGh --account work`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			if _, err := svc.UntrashFile(cmd.Context(), args[0]); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Untrashed file %s\n", args[0]); err != nil {
				return err
			}
			return nil
		},
	}
	return cmd
}
