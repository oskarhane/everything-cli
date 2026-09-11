package slack

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
)

// newUserListCmd returns `slack user list`: workspace members (users.list),
// optionally filtered client-side by a case-insensitive substring of name,
// real_name, or display_name, following cursor pagination until --max
// matches are collected (0 = no cap).
func newUserListCmd(cfg *app.Config) *cobra.Command {
	var (
		query string
		max   int
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Slack workspace members, optionally filtered by a name substring",
		Example: `# List members as JSON
everything-cli slack user list --format json

# Filter members by a name substring, as a table
everything-cli slack user list --query oskar --format table

# Return every matching member across all pages
everything-cli slack user list --query eng --max 0`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, err := dialSlack(cmd.Context(), cfg)
			if err != nil {
				return err
			}
			users, err := svc.UserList(cmd.Context(), query, max)
			if err != nil {
				return err
			}
			rows := make([]map[string]any, 0, len(users))
			for _, u := range users {
				rows = append(rows, userRow(u))
			}
			output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format), userFields, rows, rows)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&query, "query", "", "Case-insensitive substring matched against name, real_name, and display_name")
	f.IntVar(&max, "max", 25, "Maximum matching members to return (0 = no cap)")
	return cmd
}
