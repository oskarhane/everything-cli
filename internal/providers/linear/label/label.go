// Package label builds the `linear label` command tree.
package label

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
)

// NewCmd returns the `linear label` parent command with its leaves attached.
func NewCmd(cfg *app.Config, newSvc service.Dialer[service.LabelService]) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "label",
		Short: "Inspect Linear issue labels",
	}
	cmd.AddCommand(newListCmd(cfg, newSvc))
	return cmd
}
