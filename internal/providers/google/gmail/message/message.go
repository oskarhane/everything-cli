// Package message builds the `gmail message` command subtree.
package message

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/providers/google/gmail/service"
)

// NewCmd returns the `gmail message` parent with every message leaf attached.
// Each leaf lives in its own file: list.go, get.go, send.go, trash.go,
// untrash.go, delete.go, mark.go, modify.go.
func NewCmd(cfg *app.Config, newSvc service.Dialer[service.MessageService]) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "message",
		Short: "Manage Gmail messages",
	}
	cmd.AddCommand(newListCmd(cfg, newSvc))
	cmd.AddCommand(newGetCmd(cfg, newSvc))
	cmd.AddCommand(newSendCmd(cfg, newSvc))
	cmd.AddCommand(newTrashCmd(cfg, newSvc))
	cmd.AddCommand(newUntrashCmd(cfg, newSvc))
	cmd.AddCommand(newDeleteCmd(cfg, newSvc))
	cmd.AddCommand(newMarkCmd(cfg, newSvc))
	cmd.AddCommand(newModifyCmd(cfg, newSvc))
	return cmd
}
