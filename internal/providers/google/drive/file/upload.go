package file

import (
	"fmt"
	"mime"
	"path/filepath"

	drive "google.golang.org/api/drive/v3"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
)

// newUploadCmd returns `drive file upload`: create a Drive file with content
// from a local path, read through the config's afero FS.
func newUploadCmd(cfg *app.Config, newSvc service.Dialer[service.FileService]) *cobra.Command {
	var (
		name     string
		parentID string
		mimeType string
	)
	cmd := &cobra.Command{
		Use:   "upload <local-path>",
		Short: "Upload a local file to Drive",
		Example: `# Upload a file, naming it after the local base name
everything-cli google drive file upload ./report.pdf --format json

# Upload a file into a Drive folder under a new name
everything-cli google drive file upload ./report.pdf --name "Q3 report" --parent 1AbCdEfGh`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			localPath := args[0]
			content, err := cfg.Fs.Open(localPath)
			if err != nil {
				return fmt.Errorf("opening local file %s: %w", localPath, err)
			}
			defer func() { _ = content.Close() }()
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			file := &drive.File{Name: resolveUploadName(name, localPath)}
			if parentID != "" {
				file.Parents = []string{parentID}
			}
			uploaded, err := svc.UploadFile(cmd.Context(), file, resolveUploadMime(mimeType, localPath), content)
			if err != nil {
				return err
			}
			printFileView(cmd, cfg, uploaded)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&name, "name", "", "Name for the Drive file (default: the local path's base name)")
	f.StringVar(&parentID, "parent", "", "Id of the parent folder")
	f.StringVar(&mimeType, "mime-type", "", "MIME type to declare (default: from the file extension, else application/octet-stream)")
	return cmd
}

// resolveUploadName returns --name, or the local path's base name when unset.
func resolveUploadName(name, localPath string) string {
	if name != "" {
		return name
	}
	return filepath.Base(localPath)
}

// resolveUploadMime picks the upload MIME type: --mime-type wins; otherwise
// the local extension's known type, falling back to a generic binary type so
// Drive never receives an empty content type.
func resolveUploadMime(mimeType, localPath string) string {
	if mimeType != "" {
		return mimeType
	}
	if byExt := mime.TypeByExtension(filepath.Ext(localPath)); byExt != "" {
		return byExt
	}
	return "application/octet-stream"
}
