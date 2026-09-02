package skill

import (
	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/skill"
	"github.com/spf13/cobra"
)

// newPrintCmd builds skill print: echo the whole embedded bundle (SKILL.md,
// then every references/*.md in sorted order) as raw markdown.
//
// The output is written straight to the command's stdout with raw Write
// calls, deliberately bypassing output.Print: the bundle is markdown, and
// pushing it through the toon/table/json auto-detection would re-marshal
// (and corrupt) it.
func newPrintCmd(cfg *app.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "print",
		Short: "Print the embedded everything-cli skill bundle as raw markdown",
		Long: "Print the entire embedded everything-cli agent-skill bundle to stdout: " +
			"SKILL.md first, then every references/*.md in sorted order, each " +
			"preceded by a '===== references/<name>.md =====' separator line.\n\n" +
			"The output is raw markdown written directly to stdout. It deliberately " +
			"bypasses the --format auto-detection entirely (markdown must never be " +
			"marshalled through toon/table/json), so --format has no effect here.",
		Example: `# Show exactly what skill install writes on disk
everything-cli skill print

# Capture the bundle to a file
everything-cli skill print > everything-cli-skill.md`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return skill.Render(cmd.OutOrStdout())
		},
	}
}
