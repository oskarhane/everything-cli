// Package youtube builds the `youtube` command tree.
package youtube

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	yt "github.com/oskarhane/everything-cli/internal/youtube"
)

// NewCmd returns the `youtube` parent command with its leaves attached:
// transcript and metadata. Both leaves share one InnerTube client, built
// here so the command tree stays the only dialing seam.
func NewCmd(cfg *app.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "youtube",
		Short: "Fetch YouTube video metadata and transcripts",
	}
	client := yt.NewClient()
	cmd.AddCommand(newTranscriptCmd(cfg, client))
	cmd.AddCommand(newMetadataCmd(cfg, client))
	return cmd
}
