package tabs

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
)

// newDeleteCmd returns `docs tabs delete`: remove a tab — its content and
// child tabs included — by tab ID or exact title (destructive). The key is
// resolved to the tab's ID before the delete, so a title always acts on the
// tab it named when the call went out.
func newDeleteCmd(cfg *app.Config, newSvc service.Dialer[service.DocService]) *cobra.Command {
	var tab string
	cmd := &cobra.Command{
		Use:   "delete <doc-id>",
		Short: "Delete a tab by ID or exact title (destructive)",
		Example: `# Delete the tab named Appendix, its child tabs included
everything-cli google docs tabs delete 1AbCdEfGh --tab Appendix

# Same, by tab id — the way tabs list prints it
everything-cli google docs tabs delete 1AbCdEfGh --tab t.1a2b3c`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if tab == "" {
				return fmt.Errorf("--tab is required: give the ID or exact title of the tab to delete")
			}
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			resolved, err := svc.ResolveDocTab(cmd.Context(), args[0], tab)
			if err != nil {
				return err
			}
			if err := svc.DeleteDocTab(cmd.Context(), args[0], resolved.TabID); err != nil {
				return err
			}
			printDeleted(cmd, cfg, resolved)
			return nil
		},
	}
	cmd.Flags().StringVar(&tab, "tab", "", "ID or exact title of the tab to delete (required)")
	return cmd
}
