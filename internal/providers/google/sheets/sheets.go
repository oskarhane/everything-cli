// Package sheets builds the `sheets` command tree.
package sheets

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/providers/google/drive/service"
	"github.com/oskarhane/google-cli/internal/providers/google/sheets/values"
)

// sheetMetaService is the surface `sheets get` needs: spreadsheet metadata
// plus the values read that fetches each tab's header row. The concrete
// service implements both; As narrows the shared seam to this combination.
type sheetMetaService interface {
	service.SheetService
	service.SheetValuesService
}

// NewCmd returns the `sheets` parent command with its subtrees attached.
// get and delete hang directly off this parent (the resource IS the
// spreadsheet); values is a subgroup. Leaf bodies live in their own files
// in this package and in values/.
func NewCmd(cfg *app.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sheets",
		Short: "Manage Google Sheets spreadsheets and their cell values",
	}
	// The concrete service implements every drive interface; As narrows the
	// shared seam to each subtree's own surface.
	cmd.AddCommand(newGetCmd(cfg, func(ctx context.Context) (sheetMetaService, error) {
		return service.As[sheetMetaService](dial(ctx, cfg))
	}))
	cmd.AddCommand(newDeleteCmd(cfg, func(ctx context.Context) (service.FileService, error) {
		return service.As[service.FileService](dial(ctx, cfg))
	}))
	cmd.AddCommand(values.NewCmd(cfg, func(ctx context.Context) (service.SheetValuesService, error) {
		return service.As[service.SheetValuesService](dial(ctx, cfg))
	}))
	return cmd
}
