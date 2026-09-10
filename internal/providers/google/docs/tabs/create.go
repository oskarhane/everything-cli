package tabs

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
)

// newCreateCmd returns `docs tabs create`: append a new tab to the document
// and echo the tab id the API assigned it.
func newCreateCmd(cfg *app.Config, newSvc service.Dialer[service.DocService]) *cobra.Command {
	var title string
	cmd := &cobra.Command{
		Use:   "create <doc-id>",
		Short: "Add a new tab to a document",
		Example: `# Add a tab named Appendix and show the tab id it got
everything-cli google docs tabs create 1AbCdEfGh --title Appendix

# Add another tab to the same document
everything-cli google docs tabs create 1AbCdEfGh --title "Meeting notes"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if title == "" {
				return fmt.Errorf("--title is required: the title for the new tab")
			}
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			tabID, err := svc.AddDocTab(cmd.Context(), args[0], title)
			if err != nil {
				return err
			}
			printCreated(cmd, cfg, title, tabID)
			return nil
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "Title for the new tab (required)")
	return cmd
}
