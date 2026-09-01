package docs

import (
	"fmt"
	"io"
	"os"
	"path"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/subcommands/drive/service"
)

// newGetCmd returns `docs get`: the document's exported text, streamed RAW to
// stdout. The export is content, not a report, so it bypasses --format
// entirely — no table/json rendering, no control-byte stripping — and goes
// straight to stdout for piping; --out sends it to a file instead.
func newGetCmd(cfg *app.Config, newSvc service.Dialer[service.DocService]) *cobra.Command {
	var out string
	cmd := &cobra.Command{
		Use:   "get <doc-id>",
		Short: "Read a Google Doc's text content",
		Example: `# Write the document's text to a file
google-cli docs get 1AbCdEfGh --out notes.txt

# Stream the document's text to stdout for piping
google-cli docs get 1AbCdEfGh | head -20`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			text, err := svc.GetDocText(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if out == "" {
				if _, err := io.WriteString(cmd.OutOrStdout(), text); err != nil {
					return fmt.Errorf("writing document to stdout: %w", err)
				}
				return nil
			}
			return writeFile(cfg.Fs, out, text)
		},
	}
	cmd.Flags().StringVar(&out, "out", "", "Write the document's text to this file instead of stdout")
	return cmd
}

// writeFile creates the destination's parent dirs as needed, then writes the
// document text into it verbatim through the config's afero FS.
func writeFile(fs afero.Fs, out, text string) error {
	if dir := path.Dir(out); dir != "" && dir != "." {
		if err := fs.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating directory for --out %s: %w", out, err)
		}
	}
	f, err := fs.OpenFile(out, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("opening --out %s: %w", out, err)
	}
	if _, err := io.WriteString(f, text); err != nil {
		_ = f.Close()
		return fmt.Errorf("writing --out %s: %w", out, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("closing --out %s: %w", out, err)
	}
	return nil
}
