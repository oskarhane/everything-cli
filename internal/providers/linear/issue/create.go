package issue

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
)

// newCreateCmd returns `linear issue create`: create an issue in a team.
// --title is required as CLI UX even though the API's IssueCreateInput marks
// it nullable; an untitled issue is never useful output. Optional fields are
// omitted when unset, so a bare --team/--title call stays a minimal payload.
func newCreateCmd(cfg *app.Config, newSvc service.Dialer[service.IssueService], newState service.Dialer[service.StateService], newLabel service.Dialer[service.LabelService], newCycle service.Dialer[service.CycleService], newMilestone service.Dialer[service.MilestoneService]) *cobra.Command {
	var (
		teamID         string
		title          string
		description    string
		assigneeID     string
		stateValue     string
		projectID      string
		parentValue    string
		labelsValue    string
		priorityValue  string
		dueDate        string
		estimate       int
		cycleValue     string
		milestoneValue string
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a Linear issue",
		Example: `# Create an issue in a team
everything-cli linear issue create --team 9c1e2f3a-... --title "Fix login redirect"

# Create with a description, assignee, and workflow state
everything-cli linear issue create --team 9c1e2f3a-... --title "Fix login redirect" \
  --description "Users land on / after logout" --assignee 4d5e6f7a-... --state 8b9c0d1e-... \
  --project 2f4a6c8e-...

# Create a high-priority sub-issue with labels, due in the current cycle
everything-cli linear issue create --team 9c1e2f3a-... --title "Follow up on redirect" \
  --parent ENG-123 --labels "Bug, Regression" --priority high --due-date 2026-10-01 \
  --cycle "Sprint 12"`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			stateID, err := resolveStateIDForTeam(cmd.Context(), newState, teamID, stateValue)
			if err != nil {
				return err
			}
			parentID, err := resolveParentID(cmd.Context(), svc, parentValue)
			if err != nil {
				return err
			}
			labelIDs, err := resolveLabelIDs(cmd.Context(), newLabel, teamID, labelsValue)
			if err != nil {
				return err
			}
			priority, err := resolvePriority(priorityValue)
			if err != nil {
				return err
			}
			cycleID, err := resolveCycleID(cmd.Context(), newCycle, teamID, cycleValue)
			if err != nil {
				return err
			}
			milestoneID, err := resolveMilestoneID(cmd.Context(), newMilestone, projectID, milestoneValue)
			if err != nil {
				return err
			}
			issue, err := svc.CreateIssue(cmd.Context(), service.CreateIssueInput{
				TeamID:             teamID,
				Title:              title,
				Description:        description,
				AssigneeID:         assigneeID,
				StateID:            stateID,
				ProjectID:          projectID,
				ParentID:           parentID,
				LabelIDs:           labelIDs,
				Priority:           priority,
				DueDate:            dueDate,
				Estimate:           estimate,
				CycleID:            cycleID,
				ProjectMilestoneID: milestoneID,
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
	f.StringVar(&stateValue, "state", "", "Workflow state (UUID or name)")
	f.StringVar(&projectID, "project", "", "Project ID to attach the issue to")
	f.StringVar(&parentValue, "parent", "", "Parent issue (UUID or identifier like ENG-123)")
	f.StringVar(&labelsValue, "labels", "", "Comma-separated label names or UUIDs")
	f.StringVar(&priorityValue, "priority", "", "Priority: urgent, high, medium, low, none, or 0-4")
	f.StringVar(&dueDate, "due-date", "", "Due date (YYYY-MM-DD)")
	f.IntVar(&estimate, "estimate", 0, "Estimate in points")
	f.StringVar(&cycleValue, "cycle", "", "Cycle (UUID, name, or number)")
	f.StringVar(&milestoneValue, "milestone", "", "Project milestone (UUID or name; a name needs --project)")
	_ = cmd.MarkFlagRequired("team")
	_ = cmd.MarkFlagRequired("title")
	return cmd
}
