package attachment

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/subcommands/gmail/service"
)

// newGetCmd returns `gmail attachment get`: one attachment by id, fetched by
// its owning message (the attachment id only names a part of one message).
// Without --out the decoded bytes go to stdout; with --out they go to a file.
func newGetCmd(cfg *app.Config, newSvc service.Dialer[service.AttachmentService]) *cobra.Command {
	var (
		messageID string
		out       string
	)
	cmd := &cobra.Command{
		Use:   "get <attachment-id>",
		Short: "Download a Gmail attachment",
		Example: `# Write the attachment's decoded bytes to a file
google-cli gmail attachment get ANG1xQ8q --message-id 19c2a4b7 --out report.pdf

# Stream the decoded bytes to stdout for piping
google-cli gmail attachment get ANG1xQ8q --message-id 19c2a4b7 > report.pdf`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if messageID == "" {
				return fmt.Errorf("--message-id is required: an attachment id only names a part of one message")
			}
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			part, err := svc.GetAttachment(cmd.Context(), messageID, args[0])
			if err != nil {
				return err
			}
			if out == "" {
				return copyDecoded(cmd.OutOrStdout(), decodeData(part.Data), "writing attachment to stdout")
			}
			return writeFile(cfg.Fs, out, decodeData(part.Data))
		},
	}
	f := cmd.Flags()
	f.StringVar(&messageID, "message-id", "", "Id of the message the attachment belongs to (required)")
	f.StringVar(&out, "out", "", "Write the decoded bytes to this file instead of stdout")
	return cmd
}

// copyDecoded streams the decoder's output to w without ever materializing the
// full decoded attachment in memory. The streaming decoder surfaces malformed
// base64 as base64.CorruptInputError mid-copy, so it is rewrapped with a decode
// message; everything else is attributed to the destination.
func copyDecoded(w io.Writer, r io.Reader, context string) error {
	if _, err := io.Copy(w, r); err != nil {
		var corrupt base64.CorruptInputError
		if errors.As(err, &corrupt) {
			return fmt.Errorf("decoding attachment data: %w", err)
		}
		return fmt.Errorf("%s: %w", context, err)
	}
	return nil
}

// writeFile creates the destination's parent dirs as needed, then streams the
// decoded attachment bytes into it through the config's afero FS.
func writeFile(fs afero.Fs, out string, decoded io.Reader) error {
	if dir := path.Dir(out); dir != "" && dir != "." {
		if err := fs.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating directory for --out %s: %w", out, err)
		}
	}
	f, err := fs.OpenFile(out, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("opening --out %s: %w", out, err)
	}
	if err := copyDecoded(f, decoded, "writing --out "+out); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("closing --out %s: %w", out, err)
	}
	return nil
}

// decodeData returns a streaming base64url decoder over the API's attachment
// data field, which may arrive padded or unpadded: trimming the "=" padding
// lets RawURLEncoding decode both shapes byte-identically to a whole-slice
// decode. Decoded bytes flow out in chunks; nothing allocates the full payload.
// Malformed base64 surfaces as an error from the consuming copy.
func decodeData(data string) io.Reader {
	return base64.NewDecoder(base64.RawURLEncoding, strings.NewReader(strings.TrimRight(data, "=")))
}
