package label

import (
	"fmt"

	"github.com/spf13/cobra"

	gmail "google.golang.org/api/gmail/v1"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/google/gmail/service"
)

// newUpdateCmd returns `gmail label update`: modify an existing label. Only
// the flags that were set are sent, so a partial update leaves the other
// label fields untouched.
func newUpdateCmd(cfg *app.Config, newSvc service.Dialer[service.GmailService]) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a Gmail label",
		Example: `# Rename a label
everything-cli gmail label update Label_42 --name "Travel 2026"

# Recolor a label and hide its messages from the message list
everything-cli gmail label update Label_42 --color-text "#ffffff" --color-bg "#039be5" --message-list-visibility hide`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !anyLabelFlagChanged(cmd.Flags()) {
				return fmt.Errorf("nothing to update: pass at least one of --name, --color-text, --color-bg, --label-list-visibility, --message-list-visibility")
			}
			label := &gmail.Label{}
			if err := applyLabelFlags(label, cmd.Flags(), ""); err != nil {
				return err
			}
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			updated, err := svc.UpdateLabel(cmd.Context(), args[0], label)
			if err != nil {
				return err
			}
			printLabel(cmd, cfg, updated)
			return nil
		},
	}
	cmd.Flags().String("name", "", "New label name")
	addLabelFlags(cmd.Flags())
	return cmd
}
