package docs

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/providers/google/drive/service"
)

// newReplaceCmd returns `docs replace`: replace every occurrence of --find
// with --replace-with. An empty --replace-with deletes the occurrences, which
// is a legitimate use, so it is allowed; only a missing --find is an error.
func newReplaceCmd(_ *app.Config, newSvc service.Dialer[service.DocService]) *cobra.Command {
	var (
		find        string
		replaceWith string
		matchCase   bool
	)
	cmd := &cobra.Command{
		Use:   "replace <doc-id>",
		Short: "Replace text throughout a Google Doc",
		Example: `# Replace every occurrence of a name
google-cli docs replace 1AbCdEfGh --find "Project Falcon" --replace-with "Project Falcon 2"

# Replace case-insensitively (the default) and check the count
google-cli docs replace 1AbCdEfGh --find TODO --replace-with "TBD"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if find == "" {
				return fmt.Errorf("--find is required: give the text to replace")
			}
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			n, err := svc.ReplaceDocText(cmd.Context(), args[0], find, replaceWith, matchCase)
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Replaced %d occurrence(s)\n", n); err != nil {
				return err
			}
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&find, "find", "", "Text to find (required)")
	f.StringVar(&replaceWith, "replace-with", "", "Replacement text; empty deletes the matches")
	f.BoolVar(&matchCase, "match-case", false, "Match the search text case-sensitively")
	return cmd
}
