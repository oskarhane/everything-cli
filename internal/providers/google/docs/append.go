package docs

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
	"github.com/oskarhane/everything-cli/internal/providers/google/textflag"
)

// newAppendCmd returns `docs append`: add text at the very end of a tab's
// body (--tab, by tab ID or exact title; default the first tab). The text
// comes from --text or a file via --text-file, exactly one of the two, and is
// newline-terminated so successive appends each start on their own line.
func newAppendCmd(cfg *app.Config, newSvc service.Dialer[service.DocService]) *cobra.Command {
	var text, textFile, tab string
	cmd := &cobra.Command{
		Use:   "append <doc-id>",
		Short: "Append text to the end of a Google Doc",
		Example: `# Append a line to the end of a document
everything-cli google docs append 1AbCdEfGh --text "Reviewed by Oskar"

# Append the contents of a file
everything-cli google docs append 1AbCdEfGh --text-file notes.txt`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := textflag.Resolve(cfg.Fs, text, textFile, "append")
			if err != nil {
				return err
			}
			// Successive appends must each start on their own line, so a
			// missing trailing newline is added before the API call.
			if !strings.HasSuffix(body, "\n") {
				body += "\n"
			}
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			// AppendDocText resolves the tab key itself (exact tab ID,
			// then exact title); an empty tab targets the first tab.
			if err := svc.AppendDocText(cmd.Context(), args[0], body, tab); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Appended text to document %s\n", args[0]); err != nil {
				return err
			}
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&text, "text", "", "Text to append (a trailing newline is added when missing)")
	f.StringVar(&textFile, "text-file", "", "Read the text to append from this file instead of --text")
	f.StringVar(&tab, "tab", "", "Append to this tab, by tab ID or exact title (default: the first tab)")
	return cmd
}
