package docs

import (
	"fmt"
	"strings"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
)

// newAppendCmd returns `docs append`: add text at the very end of the
// document body. The text comes from --text or a file via --text-file,
// exactly one of the two, and is newline-terminated so successive appends
// each start on their own line.
func newAppendCmd(cfg *app.Config, newSvc service.Dialer[service.DocService]) *cobra.Command {
	var text, textFile string
	cmd := &cobra.Command{
		Use:   "append <doc-id>",
		Short: "Append text to the end of a Google Doc",
		Example: `# Append a line to the end of a document
everything-cli docs append 1AbCdEfGh --text "Reviewed by Oskar"

# Append the contents of a file
everything-cli docs append 1AbCdEfGh --text-file notes.txt`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := resolveText(cfg.Fs, text, textFile, "append")
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
			if err := svc.AppendDocText(cmd.Context(), args[0], body); err != nil {
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
	return cmd
}

// resolveText validates the --text / --text-file pair (exactly one) and reads
// the file variant through the config's afero FS. The text comes back
// verbatim; only append newline-terminates it (an insert must not grow the
// text by a byte the caller did not ask for).
func resolveText(fs afero.Fs, text, textFile, action string) (string, error) {
	if text != "" && textFile != "" {
		return "", fmt.Errorf("--text and --text-file are mutually exclusive")
	}
	if text == "" && textFile == "" {
		return "", fmt.Errorf("--text or --text-file is required: give the text to %s inline or via a file", action)
	}
	if textFile != "" {
		b, err := afero.ReadFile(fs, textFile)
		if err != nil {
			return "", fmt.Errorf("reading --text-file %s: %w", textFile, err)
		}
		text = string(b)
	}
	return text, nil
}
