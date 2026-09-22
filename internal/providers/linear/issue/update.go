package issue

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
)

// newUpdateCmd returns `linear issue update`: change an issue's fields.
// Only flags actually passed are sent, so unset fields are left untouched;
// passing --parent/--due-date/--cycle/--milestone with an empty string or
// --labels with an empty value clears that field on the issue.
func newUpdateCmd(cfg *app.Config, newSvc service.Dialer[service.IssueService], newState service.Dialer[service.StateService], newLabel service.Dialer[service.LabelService], newCycle service.Dialer[service.CycleService], newMilestone service.Dialer[service.MilestoneService]) *cobra.Command {
	var (
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
		Use:   "update <id>",
		Short: "Update a Linear issue",
		Example: `# Move issue BLA-123 to another workflow state
everything-cli linear issue update BLA-123 --state 8b9c0d1e-...

# Retitle and reassign in one call
everything-cli linear issue update BLA-123 --title "Fix login redirect (regression)" --assignee 4d5e6f7a-...

# Move an issue into a project
everything-cli linear issue update BLA-123 --project 2f4a6c8e-...

# Reparent, label, and prioritize; pass "" to clear a field again
everything-cli linear issue update BLA-123 --parent BLA-100 --labels "Bug" --priority urgent
everything-cli linear issue update BLA-123 --parent "" --due-date "" --labels ""`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f := cmd.Flags()
			if !f.Changed("title") && !f.Changed("description") &&
				!f.Changed("assignee") && !f.Changed("state") && !f.Changed("project") &&
				!f.Changed("parent") && !f.Changed("labels") && !f.Changed("priority") &&
				!f.Changed("due-date") && !f.Changed("estimate") && !f.Changed("cycle") &&
				!f.Changed("milestone") {
				return fmt.Errorf("nothing to update: pass at least one of --title, --description, --assignee, --state, --project, --parent, --labels, --priority, --due-date, --estimate, --cycle, --milestone")
			}
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			// team lazily fetches the issue once and returns its team ID:
			// --state, --labels, and --cycle name lookups all resolve within
			// the issue's team, and one fetch serves however many of them
			// were passed.
			var (
				current *service.Issue
				fetched bool
			)
			team := func(flag string) (string, error) {
				if !fetched {
					fetched = true
					current, err = svc.GetIssue(cmd.Context(), args[0])
					if err != nil {
						return "", err
					}
				}
				if current.Team == nil || current.Team.ID == "" {
					return "", fmt.Errorf("issue %q has no team; cannot resolve --%s by name", args[0], flag)
				}
				return current.Team.ID, nil
			}
			stateID := stateValue
			if f.Changed("state") && stateLookupRequired(stateValue) {
				teamID, err := team("state")
				if err != nil {
					return err
				}
				stateID, err = resolveStateIDForTeam(cmd.Context(), newState, teamID, stateValue)
				if err != nil {
					return err
				}
			}
			in := service.UpdateIssueInput{
				Title:       title,
				Description: description,
				AssigneeID:  assigneeID,
				StateID:     stateID,
				ProjectID:   projectID,
			}
			if f.Changed("parent") {
				parentID, err := resolveParentID(cmd.Context(), svc, parentValue)
				if err != nil {
					return err
				}
				in.ParentID = &parentID
			}
			if f.Changed("labels") {
				// "" stays a nil slice so the service sends labelIds: []
				// (clear); all-UUID values skip the team lookup entirely.
				var ids []string
				if labelLookupRequired(labelsValue) {
					teamID, err := team("labels")
					if err != nil {
						return err
					}
					ids, err = resolveLabelIDs(cmd.Context(), newLabel, teamID, labelsValue)
					if err != nil {
						return err
					}
				} else {
					ids = splitList(labelsValue)
				}
				in.LabelIDs = &ids
			}
			if f.Changed("priority") {
				priority, err := resolvePriority(priorityValue)
				if err != nil {
					return err
				}
				in.Priority = &priority
			}
			if f.Changed("due-date") {
				in.DueDate = &dueDate
			}
			if f.Changed("estimate") {
				in.Estimate = &estimate
			}
			if f.Changed("cycle") {
				cycleID := cycleValue
				if cycleValue != "" && !isUUID(cycleValue) {
					teamID, err := team("cycle")
					if err != nil {
						return err
					}
					cycleID, err = resolveCycleID(cmd.Context(), newCycle, teamID, cycleValue)
					if err != nil {
						return err
					}
				}
				in.CycleID = &cycleID
			}
			if f.Changed("milestone") {
				// Milestones resolve within a project; GetIssue's payload
				// does not carry the issue's project, so a name lookup
				// needs --project passed alongside.
				milestoneID, err := resolveMilestoneID(cmd.Context(), newMilestone, projectID, milestoneValue)
				if err != nil {
					return err
				}
				in.ProjectMilestoneID = &milestoneID
			}
			issue, err := svc.UpdateIssue(cmd.Context(), args[0], in)
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
	f.StringVar(&parentValue, "parent", "", "New parent issue (UUID or identifier; \"\" un-parents)")
	f.StringVar(&labelsValue, "labels", "", "Comma-separated label names or UUIDs (\"\" clears all labels)")
	f.StringVar(&priorityValue, "priority", "", "New priority: urgent, high, medium, low, none, or 0-4")
	f.StringVar(&dueDate, "due-date", "", "New due date, YYYY-MM-DD (\"\" clears it)")
	f.IntVar(&estimate, "estimate", 0, "New estimate in points")
	f.StringVar(&cycleValue, "cycle", "", "New cycle (UUID, name, or number; \"\" removes from the cycle)")
	f.StringVar(&milestoneValue, "milestone", "", "New project milestone (UUID or name; a name needs --project, \"\" clears it)")
	return cmd
}
