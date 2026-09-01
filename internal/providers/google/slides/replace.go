package slides

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/subcommands/drive/service"
)

// newReplaceCmd returns `slides replace`: replace every occurrence of --find
// across every slide in one API call. Matching is case-insensitive unless
// --match-case is set. The echoed count is the API's own occurrence count,
// not a local one.
func newReplaceCmd(_ *app.Config, newSvc service.Dialer[service.SlideService]) *cobra.Command {
	var (
		find        string
		replaceWith string
		matchCase   bool
	)
	cmd := &cobra.Command{
		Use:   "replace <presentation-id>",
		Short: "Replace text across every slide of a presentation",
		Example: `# Rename a client everywhere it appears
google-cli slides replace 1AbCpresentationID --find Acme --replace-with Zenith

# Replace an exact-case literal only
google-cli slides replace 1AbCpresentationID --find KPI --replace-with OKR --match-case`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if find == "" {
				return fmt.Errorf("--find is required: give the text to replace")
			}
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			count, err := svc.ReplaceSlideText(cmd.Context(), args[0], find, replaceWith, matchCase)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Replaced %d occurrence(s)\n", count)
			return err
		},
	}
	f := cmd.Flags()
	f.StringVar(&find, "find", "", "Text to find")
	f.StringVar(&replaceWith, "replace-with", "", "Replacement text")
	f.BoolVar(&matchCase, "match-case", false, "Match the find text case-sensitively")
	return cmd
}
