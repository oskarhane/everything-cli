// Package team builds the `linear team` command tree.
package team

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
)

// NewCmd returns the `linear team` parent command with its leaves attached.
func NewCmd(cfg *app.Config, newSvc service.Dialer[service.TeamService]) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "team",
		Short: "Inspect Linear teams",
	}
	cmd.AddCommand(newListCmd(cfg, newSvc))
	return cmd
}
