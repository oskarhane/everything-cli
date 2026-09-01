package docs

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/subcommands/drive/service"
)

// newInsertCmd returns `docs insert`: insert text immediately before the
// given Docs-API content index (--index, a zero-based UTF-16 code-unit
// offset, so --index 1 puts the text at the very start of the body). The
// text comes from --text or a file via --text-file, exactly one of the two,
// and is sent verbatim — no newline is added, unlike append.
func newInsertCmd(cfg *app.Config, newSvc service.Dialer[service.DocService]) *cobra.Command {
	var (
		text     string
		textFile string
		index    int64
	)
	cmd := &cobra.Command{
		Use:   "insert <doc-id>",
		Short: "Insert text into a Google Doc at a content index",
		Example: `# Insert a heading as the document's very first content
google-cli docs insert 1AbCdEfGh --index 1 --text "Q4 plan"

# Insert the contents of a file before index 120
google-cli docs insert 1AbCdEfGh --text-file block.txt --index 120`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if index <= 0 {
				return fmt.Errorf("--index is required and must be a positive Docs-API content index: the text is inserted before it")
			}
			body, err := resolveText(cfg.Fs, text, textFile, "insert")
			if err != nil {
				return err
			}
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			if err := svc.InsertDocText(cmd.Context(), args[0], body, index); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Inserted text into document %s at index %d\n", args[0], index); err != nil {
				return err
			}
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&text, "text", "", "Text to insert (inserted verbatim, before --index)")
	f.StringVar(&textFile, "text-file", "", "Read the text to insert from this file instead of --text")
	f.Int64Var(&index, "index", 0, "Docs-API content index to insert before (required, >0)")
	return cmd
}
