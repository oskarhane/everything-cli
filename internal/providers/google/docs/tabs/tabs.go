// Package tabs builds the `docs tabs` command subtree: document-tab
// management (list, add, rename, delete). Reading a tab's content is
// `docs get`; this subtree manages the tabs themselves.
package tabs

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
)

// NewCmd returns the `docs tabs` parent with every tab leaf attached. Each
// leaf lives in its own file: list.go, create.go, delete.go, rename.go.
func NewCmd(cfg *app.Config, newSvc service.Dialer[service.DocService]) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tabs",
		Short: "Manage a document's tabs (list, add, rename, delete)",
	}
	cmd.AddCommand(newListCmd(cfg, newSvc))
	cmd.AddCommand(newCreateCmd(cfg, newSvc))
	cmd.AddCommand(newDeleteCmd(cfg, newSvc))
	cmd.AddCommand(newRenameCmd(cfg, newSvc))
	return cmd
}
