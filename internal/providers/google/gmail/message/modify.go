package message

import (
	"fmt"

	"github.com/spf13/cobra"

	gmail "google.golang.org/api/gmail/v1"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/providers/google/gmail/service"
)

// newModifyCmd returns `gmail message modify`: add or remove labels on a
// message. At least one of the two lists is required.
func newModifyCmd(cfg *app.Config, newSvc service.Dialer[service.MessageService]) *cobra.Command {
	var (
		addLabelIDs    string
		removeLabelIDs string
	)
	cmd := &cobra.Command{
		Use:   "modify <id>",
		Short: "Add or remove labels on a Gmail message",
		Example: `# Add two labels and remove one
google-cli gmail message modify 19c2a4b7 --add-label-ids Label_7,Label_9 --remove-label-ids INBOX

# Remove a single label
google-cli gmail message modify 19c2a4b7 --remove-label-ids UNREAD

# See the resulting label set as JSON
google-cli gmail message modify 19c2a4b7 --add-label-ids STARRED --format json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			req := &gmail.ModifyMessageRequest{
				AddLabelIds:    SplitCSV(addLabelIDs),
				RemoveLabelIds: SplitCSV(removeLabelIDs),
			}
			if len(req.AddLabelIds) == 0 && len(req.RemoveLabelIds) == 0 {
				return fmt.Errorf("nothing to modify: pass --add-label-ids and/or --remove-label-ids")
			}
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			updated, err := svc.ModifyMessage(cmd.Context(), args[0], req)
			if err != nil {
				return err
			}
			printMessage(cmd, cfg, updated)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&addLabelIDs, "add-label-ids", "", "Comma-separated label IDs to add")
	f.StringVar(&removeLabelIDs, "remove-label-ids", "", "Comma-separated label IDs to remove")
	return cmd
}
