// Package values builds the `sheets values` command subtree.
package values

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
)

// NewCmd returns the `sheets values` parent with every values leaf attached.
// Each leaf lives in its own file: get.go, append.go, update.go, clear.go.
func NewCmd(cfg *app.Config, newSvc service.Dialer[service.SheetValuesService]) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "values",
		Short: "Read and write spreadsheet cell values by A1 range",
	}
	cmd.AddCommand(newGetCmd(cfg, newSvc))
	cmd.AddCommand(newAppendCmd(cfg, newSvc))
	cmd.AddCommand(newUpdateCmd(cfg, newSvc))
	cmd.AddCommand(newClearCmd(cfg, newSvc))
	return cmd
}
