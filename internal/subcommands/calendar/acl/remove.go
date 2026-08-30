package acl

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
)

// newRemoveCmd returns `calendar acl remove`: revoke one sharing rule.
func newRemoveCmd(_ *app.Config, newSvc serviceFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <calendar-id>",
		Short: "Revoke a calendar sharing rule",
		Example: `# Find the rule id to revoke
google-cli calendar acl list primary --format json

# Revoke a sharing rule
google-cli calendar acl remove primary --rule-id user:colleague@example.com`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ruleID, _ := cmd.Flags().GetString("rule-id")
			if ruleID == "" {
				return fmt.Errorf("--rule-id is required: the id of the sharing rule to revoke")
			}
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			return svc.DeleteAcl(cmd.Context(), args[0], ruleID)
		},
	}
	cmd.Flags().String("rule-id", "", "Id of the sharing rule to revoke")
	return cmd
}
