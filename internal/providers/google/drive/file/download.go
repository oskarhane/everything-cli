package file

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/providers/google/drive/service"
)

// newDownloadCmd returns `drive file download`: stream a file's bytes to
// stdout or --out. Google-native types (Docs/Sheets/Slides) cannot be
// downloaded as binary (files.get alt=media fails with 403
// fileNotDownloadable); they must be exported, so native types with a text
// default export on their own and the rest refuse until --export names a
// supported export MIME type.
func newDownloadCmd(cfg *app.Config, newSvc service.Dialer[service.FileService]) *cobra.Command {
	var (
		exportMime string
		out        string
	)
	cmd := &cobra.Command{
		Use:   "download <file-id>",
		Short: "Download a Drive file's content, exporting Google-native types",
		Example: `# Download a binary file to a local path
google-cli drive file download 1AbCdEfGh --out report.pdf

# Export a Google Sheet as CSV (first sheet only) to stdout for piping
google-cli drive file download 1AbCdEfGh --export text/csv > data.csv`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			stream := func(w io.Writer) error {
				return streamDownload(svc, cmd.Context(), args[0], exportMime, w)
			}
			if out == "" {
				return stream(cmd.OutOrStdout())
			}
			return app.WriteToFile(cfg.Fs, out, stream)
		},
	}
	f := cmd.Flags()
	f.StringVar(&exportMime, "export", "", "Export MIME type for Google-native files (defaults: doc -> text/plain, sheet -> text/csv — first sheet only, slide -> text/plain)")
	f.StringVar(&out, "out", "", "Write the bytes to this file instead of stdout")
	return cmd
}

// stream downloads the file's bytes into w. --export wins over everything;
// otherwise a native Google type uses its default export MIME (error naming
// the supported exports when there is none) and binary blobs stream via
// DownloadTo.
func streamDownload(svc service.FileService, ctx context.Context, fileID, exportMime string, w io.Writer) error {
	if exportMime != "" {
		return svc.ExportTo(ctx, fileID, exportMime, w)
	}
	file, err := svc.GetFile(ctx, fileID)
	if err != nil {
		return err
	}
	if export, ok := defaultExportMimes[file.MimeType]; ok {
		return svc.ExportTo(ctx, fileID, export, w)
	}
	if strings.HasPrefix(file.MimeType, "application/vnd.google-apps.") {
		return fmt.Errorf(
			"file %s is a Google-native type (%s) with no default text export; pass --export with a supported export MIME type (%s)",
			fileID, file.MimeType, supportedExports)
	}
	return svc.DownloadTo(ctx, fileID, w)
}
