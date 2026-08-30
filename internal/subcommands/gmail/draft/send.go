package draft

import (
	"github.com/spf13/cobra"

	gmail "google.golang.org/api/gmail/v1"

	"github.com/oskarhane/google-cli/internal/app"
)

// newSendCmd returns `gmail draft send`: send a stored draft by id and echo
// the sent message summary.
func newSendCmd(cfg *app.Config, newSvc serviceFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "send <id>",
		Short: "Send an existing Gmail draft",
		Example: `# Send the stored draft, echoing the sent message as JSON
google-cli gmail draft send draft_19c2a4b7 --format json

# Send the same draft, echoing it as a table
google-cli gmail draft send draft_19c2a4b7 --format table`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newDraftService(cmd.Context(), newSvc)
			if err != nil {
				return err
			}
			sent, err := svc.SendDraft(cmd.Context(), &gmail.Draft{Id: args[0]})
			if err != nil {
				return err
			}
			printSent(cmd, cfg, sent)
			return nil
		},
	}
	return cmd
}
