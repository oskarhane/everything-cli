package tabs

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
)

// newCreateCmd returns `sheets tabs create`: append a new worksheet tab and
// echo the sheet id the API assigned it.
func newCreateCmd(cfg *app.Config, newSvc service.Dialer[service.SheetTabService]) *cobra.Command {
	var title string
	cmd := &cobra.Command{
		Use:   "create <spreadsheet-id>",
		Short: "Add a new worksheet tab to a spreadsheet",
		Example: `# Add a tab named Forecast and show the sheet id it got
everything-cli google sheets tabs create 1AbCdEfGh --title Forecast

# Add another tab to the same spreadsheet
everything-cli google sheets tabs create 1AbCdEfGh --title "Actuals 2026"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if title == "" {
				return fmt.Errorf("--title is required: the title for the new tab")
			}
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			sheetID, err := svc.AddSheetTab(cmd.Context(), args[0], title)
			if err != nil {
				return err
			}
			printCreated(cmd, cfg, title, sheetID)
			return nil
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "Title for the new tab (required)")
	return cmd
}
