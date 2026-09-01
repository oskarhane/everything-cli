// Package thread builds the `gmail thread` command subtree.
package thread

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/providers/google/gmail/service"
)

// NewCmd returns the `gmail thread` parent with every thread leaf attached.
// Each leaf lives in its own file: list.go, get.go.
func NewCmd(cfg *app.Config, newSvc service.Dialer[service.ThreadService]) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "thread",
		Short: "Manage Gmail threads",
	}
	cmd.AddCommand(newListCmd(cfg, newSvc))
	cmd.AddCommand(newGetCmd(cfg, newSvc))
	return cmd
}
