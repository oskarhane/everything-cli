// Package project builds the `linear project` command tree.
package project

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
)

// NewCmd returns the `linear project` parent command with its leaves
// attached.
func NewCmd(cfg *app.Config, newSvc service.Dialer[service.ProjectService]) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Inspect Linear projects",
	}
	cmd.AddCommand(newListCmd(cfg, newSvc))
	return cmd
}
