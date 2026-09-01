package issue

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
)

// newUpdateCmd returns `linear issue update`: change an issue's title,
// description, assignee, or state. Only flags actually passed are sent, so
// unset fields are left untouched.
func newUpdateCmd(cfg *app.Config, newSvc service.Dialer[service.IssueService]) *cobra.Command {
	var (
		title       string
		description string
		assigneeID  string
		stateID     string
	)
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a Linear issue",
		Example: `# Move issue BLA-123 to another workflow state
everything-cli linear issue update BLA-123 --state 8b9c0d1e-...

# Retitle and reassign in one call
everything-cli linear issue update BLA-123 --title "Fix login redirect (regression)" --assignee 4d5e6f7a-...`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f := cmd.Flags()
			if !f.Changed("title") && !f.Changed("description") &&
				!f.Changed("assignee") && !f.Changed("state") {
				return fmt.Errorf("nothing to update: pass at least one of --title, --description, --assignee, --state")
			}
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			issue, err := svc.UpdateIssue(cmd.Context(), args[0], service.UpdateIssueInput{
				Title:       title,
				Description: description,
				AssigneeID:  assigneeID,
				StateID:     stateID,
			})
			if err != nil {
				return err
			}
			printIssue(cmd, cfg, issue)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&title, "title", "", "New issue title")
	f.StringVar(&description, "description", "", "New issue description (markdown)")
	f.StringVar(&assigneeID, "assignee", "", "New assignee user ID")
	f.StringVar(&stateID, "state", "", "New workflow state ID")
	return cmd
}
