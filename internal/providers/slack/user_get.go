package slack

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
)

// newUserGetCmd returns `slack user get`: one workspace member by Slack user
// ID (users.info).
func newUserGetCmd(cfg *app.Config) *cobra.Command {
	var userID string
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Show one Slack workspace member",
		Example: `# Show a member as JSON
everything-cli slack user get --user U02H6ECK2 --format json

# Show a member as a table
everything-cli slack user get --user U02H6ECK2 --format table`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, err := dialSlack(cmd.Context(), cfg)
			if err != nil {
				return err
			}
			user, err := svc.UserGet(cmd.Context(), userID)
			if err != nil {
				return err
			}
			output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format), userFields,
				user, []map[string]any{userRow(user)})
			return nil
		},
	}
	cmd.Flags().StringVar(&userID, "user", "", "Slack user ID (e.g. U02H6ECK2)")
	_ = cmd.MarkFlagRequired("user")
	return cmd
}
