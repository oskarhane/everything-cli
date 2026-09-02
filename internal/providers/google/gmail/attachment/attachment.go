// Package attachment builds the `gmail attachment` command subtree.
package attachment

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/google/gmail/service"
)

// NewCmd returns the `gmail attachment` parent with every attachment leaf
// attached. Each leaf lives in its own file: get.go.
func NewCmd(cfg *app.Config, newSvc service.Dialer[service.AttachmentService]) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "attachment",
		Short: "Manage Gmail attachments",
	}
	cmd.AddCommand(newGetCmd(cfg, newSvc))
	return cmd
}
