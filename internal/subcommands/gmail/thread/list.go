package thread

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
)

// newListCmd returns `gmail thread list`: threads matching a Gmail search
// query, with --label-ids narrowing results to threads whose messages carry
// the labels.
func newListCmd(cfg *app.Config, newSvc serviceFunc) *cobra.Command {
	var (
		query      string
		labelIDs   string
		maxResults int64
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Gmail threads",
		Example: `# List the 25 most recent threads as JSON
google-cli gmail thread list --format json

# Search threads mentioning an invoice, at most 10, with a label, as a table
google-cli gmail thread list --query "subject:invoice" --label-ids Label_7 --max 10 --format table`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, err := newThreadService(cmd.Context(), newSvc)
			if err != nil {
				return err
			}
			threads, err := svc.ListThreads(cmd.Context(), query, splitCSV(labelIDs), maxResults)
			if err != nil {
				return err
			}
			printThreads(cmd, cfg, threads)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVarP(&query, "query", "q", "", "Gmail search query (e.g. \"from:boss@corp.com subject:invoice\")")
	f.StringVar(&labelIDs, "label-ids", "", "Comma-separated label IDs every result must carry")
	f.Int64Var(&maxResults, "max", 25, "Maximum threads to return")
	return cmd
}

// splitCSV splits a comma-separated flag value into trimmed, non-empty items,
// the CSV convention for the --label-ids flag.
func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	var items []string
	for _, part := range strings.Split(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			items = append(items, p)
		}
	}
	return items
}
