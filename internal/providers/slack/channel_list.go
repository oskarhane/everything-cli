package slack

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
)

// defaultChannelTypes is the --types default: every conversation kind a user
// token can read, matching the set the account-level history is usually
// scoped to.
const defaultChannelTypes = "public_channel,private_channel,im,mpim"

// channelListFields is the table column order; the same snake_case names are
// the JSON and TOON keys. go-pretty's StyleLight upper-cases the header cells
// at render time.
var channelListFields = []string{"id", "name", "is_private", "user"}

// newChannelListCmd returns `slack channel list`: the conversations visible to
// the token, following cursor pagination up to --max.
func newChannelListCmd(cfg *app.Config) *cobra.Command {
	var opts ChannelListOptions
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Slack conversations",
		Example: `# List every conversation type as JSON
everything-cli slack channel list --format json

# List public and private channels only, as a table
everything-cli slack channel list --types public_channel,private_channel --format table`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, err := dialSlack(cmd.Context(), cfg)
			if err != nil {
				return err
			}
			channels, err := svc.ChannelList(cmd.Context(), opts)
			if err != nil {
				return err
			}
			printChannelList(cmd, cfg, channels)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&opts.Types, "types", defaultChannelTypes, "Comma-separated conversation types: public_channel,private_channel,im,mpim")
	f.IntVar(&opts.Max, "max", defaultChannelMax, "Total max channels across all pages (0 = no cap)")
	return cmd
}

// printChannelList renders channels in the resolved format: JSON/TOON as the
// shared Channel shape, or one table row per conversation.
func printChannelList(cmd *cobra.Command, cfg *app.Config, channels []Channel) {
	rows := make([]map[string]any, 0, len(channels))
	for _, c := range channels {
		rows = append(rows, channelListRow(c))
	}
	output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format), channelListFields, rows, rows)
}

// channelListRow maps one channel to its output row, mirroring the shared
// Channel tags: user is the counterpart for im entries and omitted elsewhere.
func channelListRow(c Channel) map[string]any {
	row := map[string]any{
		"id":         c.ID,
		"name":       c.Name,
		"is_private": c.IsPrivate,
	}
	if c.User != "" {
		row["user"] = c.User
	}
	return row
}
