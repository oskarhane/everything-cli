package tabs

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
)

// newListCmd returns `docs tabs list`: the document's tabs flattened
// depth-first (a parent before its child tabs), so the order reads like the
// document's own tab sidebar. The tab ids it prints are the keys every
// --tab flag here accepts.
func newListCmd(cfg *app.Config, newSvc service.Dialer[service.DocService]) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list <doc-id>",
		Short: "List a document's tabs, child tabs under their parents",
		Example: `# List a document's tabs as JSON
everything-cli google docs tabs list 1AbCdEfGh --format json

# Show the same tabs as a table
everything-cli google docs tabs list 1AbCdEfGh --format table

# Find the tab id a --tab flag needs
everything-cli google docs tabs list 1AbCdEfGh --format json | jq '.[] | select(.title=="Appendix")'`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			docTabs, err := svc.ListDocTabs(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			printTabList(cmd, cfg, docTabs)
			return nil
		},
	}
	return cmd
}
