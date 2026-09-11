package slack

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
)

// newFileDownloadCmd returns `slack file download`: stream one file's bytes
// to stdout or --out. It resolves the file via files.info, then streams the
// authenticated url_private (url_private itself is never printed).
func newFileDownloadCmd(cfg *app.Config) *cobra.Command {
	var out string
	cmd := &cobra.Command{
		Use:   "download <file-id>",
		Short: "Download a Slack file's content",
		Example: `# Stream a file to stdout for piping
everything-cli slack file download F0B3HMXFEUV > report.pdf

# Write a file to a local path
everything-cli slack file download F0B3HMXFEUV --out report.pdf`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := dialSlack(cmd.Context(), cfg)
			if err != nil {
				return err
			}
			stream := func(w io.Writer) error {
				return svc.DownloadFileTo(cmd.Context(), args[0], w)
			}
			if out == "" {
				return stream(cmd.OutOrStdout())
			}
			return app.WriteToFile(cfg.Fs, out, stream)
		},
	}
	cmd.Flags().StringVar(&out, "out", "", "Write the bytes to this file instead of stdout")
	return cmd
}
