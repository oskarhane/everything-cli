package file

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/providers/google/drive/service"
)

// newGetCmd returns `drive file get`: one file's metadata by id. Metadata
// only — content downloads go through download.
func newGetCmd(cfg *app.Config, newSvc service.Dialer[service.FileService]) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <file-id>",
		Short: "Show a Drive file's metadata",
		Example: `# Show a file's metadata as JSON
google-cli drive file get 1AbCdEfGh --format json

# Show the same file as a table
google-cli drive file get 1AbCdEfGh --format table`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			file, err := svc.GetFile(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			printFileView(cmd, cfg, file)
			return nil
		},
	}
	return cmd
}
