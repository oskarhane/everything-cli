package message

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/google/gmail/service"
)

// newListCmd returns `gmail message list`: messages matching a Gmail search
// query, composed client-side from --query, --label-ids, and --unread-only.
func newListCmd(cfg *app.Config, newSvc service.Dialer[service.MessageService]) *cobra.Command {
	var (
		query      string
		labelIDs   string
		maxResults int64
		unreadOnly bool
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Gmail messages",
		Example: `# List the 25 most recent inbox messages as JSON
everything-cli gmail message list --query "label:INBOX" --format json

# List at most 10 unread messages with a label, as a table
everything-cli gmail message list --label-ids Label_7 --unread-only --max 10 --format table`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			q := composeQuery(query, SplitCSV(labelIDs), unreadOnly)
			messages, err := svc.ListMessages(cmd.Context(), q, maxResults)
			if err != nil {
				return err
			}
			printMessages(cmd, cfg, messages)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVarP(&query, "query", "q", "", "Gmail search query (e.g. \"from:boss@corp.com subject:invoice\")")
	f.StringVar(&labelIDs, "label-ids", "", "Comma-separated label IDs every result must carry")
	f.Int64Var(&maxResults, "max", 25, "Maximum messages to return")
	f.BoolVar(&unreadOnly, "unread-only", false, "Only unread messages")
	return cmd
}

// composeQuery builds the API's q parameter: the raw query plus label: terms
// plus is:unread, space-joined (Gmail search ANDs space-separated terms).
func composeQuery(query string, labelIDs []string, unreadOnly bool) string {
	var terms []string
	if query != "" {
		terms = append(terms, query)
	}
	for _, id := range labelIDs {
		terms = append(terms, "label:"+id)
	}
	if unreadOnly {
		terms = append(terms, "is:unread")
	}
	return strings.Join(terms, " ")
}
