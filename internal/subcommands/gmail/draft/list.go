package draft

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
)

// newListCmd returns `gmail draft list`: the stored drafts, newest first.
func newListCmd(cfg *app.Config, newSvc serviceFunc) *cobra.Command {
	var maxResults int64
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Gmail drafts",
		Example: `# List the 25 most recent drafts as JSON
google-cli gmail draft list --format json

# List at most 10 drafts as a table
google-cli gmail draft list --max 10 --format table`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, err := newDraftService(cmd.Context(), newSvc)
			if err != nil {
				return err
			}
			drafts, err := svc.ListDrafts(cmd.Context(), maxResults)
			if err != nil {
				return err
			}
			printDrafts(cmd, cfg, drafts)
			return nil
		},
	}
	cmd.Flags().Int64Var(&maxResults, "max", 25, "Maximum drafts to return")
	return cmd
}
