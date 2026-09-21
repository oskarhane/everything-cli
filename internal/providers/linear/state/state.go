// Package state builds the `linear state` command tree.
package state

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
)

// NewCmd returns the `linear state` parent command with its leaves attached.
func NewCmd(cfg *app.Config, newSvc service.Dialer[service.StateService]) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "state",
		Short: "Inspect Linear workflow states",
	}
	cmd.AddCommand(newListCmd(cfg, newSvc))
	return cmd
}
