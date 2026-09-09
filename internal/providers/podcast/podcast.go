// Package podcast builds the `podcast` command tree.
package podcast

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	podcastapi "github.com/oskarhane/everything-cli/internal/podcast"
)

// NewCmd returns the `podcast` parent command with its leaves attached:
// transcript. The leaf shares one Client, built here so the command tree
// stays the only dialing seam.
func NewCmd(cfg *app.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   ID,
		Short: "Fetch podcast episode transcripts",
	}
	client := podcastapi.NewClient()
	cmd.AddCommand(newTranscriptCmd(cfg, client))
	return cmd
}
