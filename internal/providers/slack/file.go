package slack

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
)

// newFileCmd builds the `slack file` resource tree: `file download` streams
// one uploaded file's bytes to stdout or --out (files.info + url_private).
func newFileCmd(cfg *app.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "file",
		Short: "Download Slack files",
	}
	cmd.AddCommand(newFileDownloadCmd(cfg))
	return cmd
}
