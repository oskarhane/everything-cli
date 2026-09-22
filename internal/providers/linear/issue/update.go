package issue

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
)

// newUpdateCmd returns `linear issue update`: change an issue's title,
// description, assignee, state, or project. Only flags actually passed are
// sent, so unset fields are left untouched.
func newUpdateCmd(cfg *app.Config, newSvc service.Dialer[service.IssueService], newState service.Dialer[service.StateService]) *cobra.Command {
	var (
		title       string
		description string
		assigneeID  string
		stateValue  string
		projectID   string
	)
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a Linear issue",
		Example: `# Move issue BLA-123 to another workflow state
everything-cli linear issue update BLA-123 --state 8b9c0d1e-...

# Retitle and reassign in one call
everything-cli linear issue update BLA-123 --title "Fix login redirect (regression)" --assignee 4d5e6f7a-...

# Move an issue into a project
everything-cli linear issue update BLA-123 --project 2f4a6c8e-...`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f := cmd.Flags()
			if !f.Changed("title") && !f.Changed("description") &&
				!f.Changed("assignee") && !f.Changed("state") && !f.Changed("project") {
				return fmt.Errorf("nothing to update: pass at least one of --title, --description, --assignee, --state, --project")
			}
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			stateID := stateValue
			if f.Changed("state") && stateLookupRequired(stateValue) {
				current, err := svc.GetIssue(cmd.Context(), args[0])
				if err != nil {
					return err
				}
				if current.Team == nil || current.Team.ID == "" {
					return fmt.Errorf("issue %q has no team; cannot resolve --state by name", args[0])
				}
				stateID, err = resolveStateIDForTeam(cmd.Context(), newState, current.Team.ID, stateValue)
				if err != nil {
					return err
				}
			}
			issue, err := svc.UpdateIssue(cmd.Context(), args[0], service.UpdateIssueInput{
				Title:       title,
				Description: description,
				AssigneeID:  assigneeID,
				StateID:     stateID,
				ProjectID:   projectID,
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
	f.StringVar(&stateValue, "state", "", "New workflow state (UUID or name)")
	f.StringVar(&projectID, "project", "", "New project ID")
	return cmd
}
