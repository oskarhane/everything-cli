package file

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
)

// newDownloadCmd returns `linear file download`: stream one Linear
// file-storage URL's bytes to stdout or --out. Storage files are private —
// the service's authenticated client carries the acting account's
// credential — and only https://uploads.linear.app is fetched. A failed
// download with --out removes the destination, so no zero-byte or partial
// file is left behind.
func newDownloadCmd(cfg *app.Config, newSvc service.Dialer[service.FileService]) *cobra.Command {
	var out string
	cmd := &cobra.Command{
		Use:   "download <url>",
		Short: "Download a Linear file-storage file",
		Example: `# Stream a file to stdout for piping
everything-cli linear file download https://uploads.linear.app/8c8a1b2c-0000-4000-8000-000000000001 > screenshot.png

# Write a file to a local path
everything-cli linear file download https://uploads.linear.app/8c8a1b2c-0000-4000-8000-000000000001 --out screenshot.png`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			stream := func(w io.Writer) error {
				return svc.DownloadFileTo(cmd.Context(), args[0], w)
			}
			if out == "" {
				return stream(cmd.OutOrStdout())
			}
			if err := app.WriteToFile(cfg.Fs, out, stream); err != nil {
				// WriteToFile truncates or creates the destination before
				// streaming, so a failed download — a refused URL, a
				// non-200, a mid-stream failure — must remove what it
				// left rather than keep a zero-byte or partial file.
				_ = cfg.Fs.Remove(out)
				return err
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&out, "out", "", "Write the bytes to this file instead of stdout")
	return cmd
}
