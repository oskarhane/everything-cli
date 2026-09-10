// Package tabs builds the `sheets tabs` command subtree: worksheet-tab
// management (add, rename, delete). Listing the tabs is `sheets get`; cell
// values are `sheets values` — this subtree is management only, and its
// leaves are writes, so none carries --format.
package tabs

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
)

// NewCmd returns the `sheets tabs` parent with every tab leaf attached.
// Each leaf lives in its own file: create.go, delete.go, rename.go.
func NewCmd(cfg *app.Config, newSvc service.Dialer[service.SheetTabService]) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tabs",
		Short: "Manage a spreadsheet's worksheet tabs (add, rename, delete)",
	}
	cmd.AddCommand(newCreateCmd(cfg, newSvc))
	cmd.AddCommand(newDeleteCmd(cfg, newSvc))
	cmd.AddCommand(newRenameCmd(cfg, newSvc))
	return cmd
}
