package draft

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
)

// newGetCmd returns `gmail draft get`: one stored draft by id, showing its
// id and the stored message's headers and snippet.
func newGetCmd(cfg *app.Config, newSvc serviceFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Show a Gmail draft by id",
		Example: `# Show a draft's stored message headers as JSON
google-cli gmail draft get draft_19c2a4b7 --format json

# Show the same draft as a table
google-cli gmail draft get draft_19c2a4b7 --format table`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newDraftService(cmd.Context(), newSvc)
			if err != nil {
				return err
			}
			draft, err := svc.GetDraft(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			printDraftDetail(cmd, cfg, draft)
			return nil
		},
	}
	return cmd
}
