package tabs

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
)

// newRenameCmd returns `docs tabs rename`: retitle a tab matched by tab ID
// or exact current title.
func newRenameCmd(cfg *app.Config, newSvc service.Dialer[service.DocService]) *cobra.Command {
	var tab, title string
	cmd := &cobra.Command{
		Use:   "rename <doc-id>",
		Short: "Rename a tab",
		Example: `# Rename the tab Appendix to "Appendix v2"
everything-cli google docs tabs rename 1AbCdEfGh --tab Appendix --title "Appendix v2"

# Same, by tab id — the way tabs list prints it
everything-cli google docs tabs rename 1AbCdEfGh --tab t.1a2b3c --title "Appendix v2"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if tab == "" {
				return fmt.Errorf("--tab is required: give the ID or exact current title of the tab to rename")
			}
			if title == "" {
				return fmt.Errorf("--title is required: the tab's new title")
			}
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			resolved, err := svc.ResolveDocTab(cmd.Context(), args[0], tab)
			if err != nil {
				return err
			}
			if err := svc.RenameDocTab(cmd.Context(), args[0], resolved.TabID, title); err != nil {
				return err
			}
			printRenamed(cmd, cfg, resolved.Title, title)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&tab, "tab", "", "ID or exact current title of the tab to rename (required)")
	f.StringVar(&title, "title", "", "New title for the tab (required)")
	return cmd
}
