package tabs

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
)

// newRenameCmd returns `sheets tabs rename`: retitle a worksheet tab,
// matched by its exact current title.
func newRenameCmd(cfg *app.Config, newSvc service.Dialer[service.SheetTabService]) *cobra.Command {
	var tab, title string
	cmd := &cobra.Command{
		Use:   "rename <spreadsheet-id>",
		Short: "Rename a worksheet tab",
		Example: `# Rename the tab Notes to Archive
everything-cli google sheets tabs rename 1AbCdEfGh --tab Notes --title Archive

# Titles match exactly: this fails when the tab is "notes"
everything-cli google sheets tabs rename 1AbCdEfGh --tab notes --title Archive`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if title == "" {
				return fmt.Errorf("--title is required: the tab's new title")
			}
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			if err := svc.RenameSheetTab(cmd.Context(), args[0], tab, title); err != nil {
				return err
			}
			printRenamed(cmd, cfg, tab, title)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&tab, "tab", "", "Current title of the tab to rename, matched exactly (required)")
	f.StringVar(&title, "title", "", "New title for the tab (required)")
	return cmd
}
