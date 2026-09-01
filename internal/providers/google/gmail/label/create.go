package label

import (
	"github.com/spf13/cobra"

	gmail "google.golang.org/api/gmail/v1"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/providers/google/gmail/service"
)

// newCreateCmd returns `gmail label create`: a new label.
func newCreateCmd(cfg *app.Config, newSvc service.Dialer[service.GmailService]) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a Gmail label",
		Example: `# Create a label
google-cli gmail label create Travel

# Create a colored label, hidden from the label list
google-cli gmail label create Travel --color-text "#ffffff" --color-bg "#039be5" --label-list-visibility labelHide`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			label := &gmail.Label{}
			if err := applyLabelFlags(label, cmd.Flags(), args[0]); err != nil {
				return err
			}
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			created, err := svc.CreateLabel(cmd.Context(), label)
			if err != nil {
				return err
			}
			printLabel(cmd, cfg, created)
			return nil
		},
	}
	addLabelFlags(cmd.Flags())
	return cmd
}
