package tabs

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
)

// newDeleteCmd returns `sheets tabs delete`: remove a worksheet tab — every
// cell on it included — by its exact title (destructive).
func newDeleteCmd(cfg *app.Config, newSvc service.Dialer[service.SheetTabService]) *cobra.Command {
	var tab string
	cmd := &cobra.Command{
		Use:   "delete <spreadsheet-id>",
		Short: "Delete a worksheet tab by title (destructive)",
		Example: `# Delete the tab named Notes, every cell on it included
everything-cli google sheets tabs delete 1AbCdEfGh --tab Notes

# Titles match exactly: this fails when the tab is "notes"
everything-cli google sheets tabs delete 1AbCdEfGh --tab notes`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if tab == "" {
				return fmt.Errorf("--tab is required: give the exact title of the tab to delete")
			}
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			if err := svc.DeleteSheetTab(cmd.Context(), args[0], tab); err != nil {
				return err
			}
			printDeleted(cmd, cfg, tab)
			return nil
		},
	}
	cmd.Flags().StringVar(&tab, "tab", "", "Title of the tab to delete, matched exactly (required)")
	return cmd
}
