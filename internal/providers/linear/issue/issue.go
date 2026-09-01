// Package issue builds the `linear issue` command tree.
package issue

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/providers/linear/service"
)

// NewCmd returns the `linear issue` parent command with its leaves
// attached. Every leaf lives in its own file with one AddCommand line per
// leaf.
func NewCmd(cfg *app.Config, newSvc service.Dialer[service.IssueService]) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "issue",
		Short: "Manage Linear issues",
	}
	cmd.AddCommand(newListCmd(cfg, newSvc))
	cmd.AddCommand(newGetCmd(cfg, newSvc))
	cmd.AddCommand(newCreateCmd(cfg, newSvc))
	cmd.AddCommand(newUpdateCmd(cfg, newSvc))
	return cmd
}
