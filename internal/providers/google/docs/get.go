package docs

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
)

// newGetCmd returns `docs get`: the document's exported text, streamed RAW to
// stdout. The export is content, not a report, so it bypasses --format
// entirely — no table/json rendering, no control-byte stripping — and goes
// straight to stdout for piping; --out sends it to a file instead. --tab
// reads a single tab's rendered text instead of the doc-wide export, through
// the same raw plumbing.
func newGetCmd(cfg *app.Config, newSvc service.Dialer[service.DocService]) *cobra.Command {
	var out, tab string
	cmd := &cobra.Command{
		Use:   "get <doc-id>",
		Short: "Read a Google Doc's text content",
		Example: `# Write the document's text to a file
everything-cli google docs get 1AbCdEfGh --out notes.txt

# Stream the document's text to stdout for piping
everything-cli google docs get 1AbCdEfGh | head -20`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			// --tab reads one tab's render (the service resolves the key:
			// exact tab ID, then exact title); the default stays the
			// doc-wide Drive export. Both are raw content — the same
			// --out/stdout plumbing, no format handling on either.
			var text string
			if tab == "" {
				text, err = svc.GetDocText(cmd.Context(), args[0])
			} else {
				text, err = svc.GetDocTabText(cmd.Context(), args[0], tab)
			}
			if err != nil {
				return err
			}
			if out == "" {
				if _, err := io.WriteString(cmd.OutOrStdout(), text); err != nil {
					return fmt.Errorf("writing document to stdout: %w", err)
				}
				return nil
			}
			return app.WriteToFile(cfg.Fs, out, func(w io.Writer) error {
				_, err := io.WriteString(w, text)
				return err
			})
		},
	}
	cmd.Flags().StringVar(&out, "out", "", "Write the document's text to this file instead of stdout")
	cmd.Flags().StringVar(&tab, "tab", "", "Read only this tab, by tab ID or exact title (default: the whole document)")
	return cmd
}
