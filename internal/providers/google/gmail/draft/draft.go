// Package draft builds the `gmail draft` command subtree.
package draft

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/providers/google/gmail/service"
)

// NewCmd returns the `gmail draft` parent with every draft leaf attached.
// Each leaf lives in its own file: list.go, get.go, create.go, send.go,
// delete.go.
func NewCmd(cfg *app.Config, newSvc service.Dialer[service.DraftService]) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "draft",
		Short: "Manage Gmail drafts",
	}
	cmd.AddCommand(newListCmd(cfg, newSvc))
	cmd.AddCommand(newGetCmd(cfg, newSvc))
	cmd.AddCommand(newCreateCmd(cfg, newSvc))
	cmd.AddCommand(newSendCmd(cfg, newSvc))
	cmd.AddCommand(newDeleteCmd(cfg, newSvc))
	return cmd
}
