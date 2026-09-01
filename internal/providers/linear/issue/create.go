package issue

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/providers/linear/service"
)

// newCreateCmd returns `linear issue create`: create an issue in a team.
// --title is required as CLI UX even though the API's IssueCreateInput marks
// it nullable; an untitled issue is never useful output.
func newCreateCmd(cfg *app.Config, newSvc service.Dialer[service.IssueService]) *cobra.Command {
	var (
		teamID      string
		title       string
		description string
		assigneeID  string
		stateID     string
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a Linear issue",
		Example: `# Create an issue in a team
google-cli linear issue create --team 9c1e2f3a-... --title "Fix login redirect"

# Create with a description, assignee, and workflow state
google-cli linear issue create --team 9c1e2f3a-... --title "Fix login redirect" \
  --description "Users land on / after logout" --assignee 4d5e6f7a-... --state 8b9c0d1e-...`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			issue, err := svc.CreateIssue(cmd.Context(), service.CreateIssueInput{
				TeamID:      teamID,
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
	f.StringVar(&teamID, "team", "", "Team ID the issue belongs to (required)")
	f.StringVar(&title, "title", "", "Issue title (required)")
	f.StringVar(&description, "description", "", "Issue description (markdown)")
	f.StringVar(&assigneeID, "assignee", "", "Assignee user ID")
	f.StringVar(&stateID, "state", "", "Workflow state ID")
	_ = cmd.MarkFlagRequired("team")
	_ = cmd.MarkFlagRequired("title")
	return cmd
}
