package slack

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
)

// userFields is the workspace-member output field order for table output; the
// same snake_case names are the JSON and TOON keys. `user get` and
// `user list` share the shape so agents can move between them unchanged.
var userFields = []string{"id", "name", "real_name", "display_name"}

// userRow maps one User into its output row.
func userRow(u User) map[string]any {
	return map[string]any{
		"id":           u.ID,
		"name":         u.Name,
		"real_name":    u.RealName,
		"display_name": u.DisplayName,
	}
}

// newUserCmd builds the `slack user` resource tree: `user get` looks up one
// member by ID (users.info), `user list` pages and filters the workspace
// directory (users.list).
func newUserCmd(cfg *app.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user",
		Short: "Look up Slack workspace members",
	}
	cmd.AddCommand(newUserGetCmd(cfg))
	cmd.AddCommand(newUserListCmd(cfg))
	return cmd
}
