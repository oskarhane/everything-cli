package issue

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
)

// newSearchCmd returns `linear issue search`: full-text search over issue
// titles, descriptions, and comments, ranked by relevance. Results render
// through the shared issue-list surface.
func newSearchCmd(cfg *app.Config, newSvc service.Dialer[service.SearchService]) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search Linear issues by title, description, and comments",
		Example: `# Search issues as JSON
everything-cli linear issue search --query "login redirect" --format json

# Search issues as a table
everything-cli linear issue search --query "login redirect" --format table`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			query, _ := cmd.Flags().GetString("query")
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			issues, err := svc.SearchIssues(cmd.Context(), query)
			if err != nil {
				return err
			}
			printIssueList(cmd, cfg, issues)
			return nil
		},
	}
	cmd.Flags().String("query", "", "Full-text search query (matches title, description, and comments)")
	_ = cmd.MarkFlagRequired("query")
	return cmd
}
