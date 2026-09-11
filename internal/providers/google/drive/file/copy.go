package file

import (
	"github.com/spf13/cobra"

	drive "google.golang.org/api/drive/v3"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
)

// newCopyCmd returns `drive file copy`: duplicate a file, optionally renaming
// it and/or placing the copy in a different parent folder. Drive keeps the
// source's MIME type on a copy, so Google-native Docs/Sheets/Slides stay
// native — the copy leaf never sends a mimeType override.
func newCopyCmd(cfg *app.Config, newSvc service.Dialer[service.FileService]) *cobra.Command {
	var (
		name     string
		parentID string
	)
	cmd := &cobra.Command{
		Use:   "copy <file-id>",
		Short: "Copy a Drive file, optionally renaming it or moving the copy to a folder",
		Example: `# Copy a file, keeping its name and parent
everything-cli google drive file copy 1AbCdEfGh --format json

# Copy a file under a new name into a folder
everything-cli google drive file copy 1AbCdEfGh --name "Q3 report" --parent 1AbCdEfGh`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			copied, err := svc.CopyFile(cmd.Context(), args[0], copyMetadata(name, parentID))
			if err != nil {
				return err
			}
			printFileView(cmd, cfg, copied)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&name, "name", "", "New name for the copy (default: the source file's name)")
	f.StringVar(&parentID, "parent", "", "Id of the parent folder for the copy (default: the source file's parent)")
	return cmd
}

// copyMetadata builds the files.copy metadata: the name and/or parent to
// apply to the copy. MimeType is deliberately never set — Drive keeps the
// source's type, which is what preserves a Google-native file's native type.
func copyMetadata(name, parentID string) *drive.File {
	f := &drive.File{}
	if name != "" {
		f.Name = name
	}
	if parentID != "" {
		f.Parents = []string{parentID}
	}
	return f
}
