package issue

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/providers/linear/service"
)

// newGetCmd returns `linear issue get`: one issue by UUID or human
// identifier ("BLA-123").
func newGetCmd(cfg *app.Config, newSvc service.Dialer[service.IssueService]) *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Show a Linear issue",
		Example: `# Show issue BLA-123 as JSON
google-cli linear issue get BLA-123 --format json

# Show the same issue as a table
google-cli linear issue get BLA-123 --format table`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			issue, err := svc.GetIssue(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			printIssue(cmd, cfg, issue)
			return nil
		},
	}
}
