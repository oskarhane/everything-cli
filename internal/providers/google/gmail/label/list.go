package label

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/providers/google/gmail/service"
)

// newListCmd returns `gmail label list`: every label on the account.
func newListCmd(cfg *app.Config, newSvc service.Dialer[service.GmailService]) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List Gmail labels",
		Example: `# List labels as JSON
google-cli gmail label list --format json

# List labels as a table
google-cli gmail label list --format table`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			labels, err := svc.ListLabels(cmd.Context())
			if err != nil {
				return err
			}
			printLabels(cmd, cfg, labels)
			return nil
		},
	}
}
