package attachment

import (
	"encoding/base64"
	"fmt"
	"path"
	"strings"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
)

// newGetCmd returns `gmail attachment get`: one attachment by id, fetched by
// its owning message (the attachment id only names a part of one message).
// Without --out the decoded bytes go to stdout; with --out they go to a file.
func newGetCmd(cfg *app.Config, newSvc serviceFunc) *cobra.Command {
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
			svc, err := newAttachmentService(cmd.Context(), newSvc)
			if err != nil {
				return err
			}
			part, err := svc.GetAttachment(cmd.Context(), messageID, args[0])
			if err != nil {
				return err
			}
			content, err := decodeData(part.Data)
			if err != nil {
				return err
			}
			if out == "" {
				_, err := cmd.OutOrStdout().Write(content)
				return err
			}
			return writeFile(cfg.Fs, out, content)
		},
	}
	f := cmd.Flags()
	f.StringVar(&messageID, "message-id", "", "Id of the message the attachment belongs to (required)")
	f.StringVar(&out, "out", "", "Write the decoded bytes to this file instead of stdout")
	return cmd
}

// writeFile creates the destination's parent dirs as needed, then writes the
// decoded attachment bytes through the config's afero FS.
func writeFile(fs afero.Fs, out string, content []byte) error {
	if dir := path.Dir(out); dir != "" && dir != "." {
		if err := fs.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating directory for --out %s: %w", out, err)
		}
	}
	if err := afero.WriteFile(fs, out, content, 0o644); err != nil {
		return fmt.Errorf("writing --out %s: %w", out, err)
	}
	return nil
}

// decodeData decodes the API's base64url attachment data field, which may
// arrive padded or unpadded.
func decodeData(data string) ([]byte, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(data, "="))
	if err != nil {
		return nil, fmt.Errorf("decoding attachment data: %w", err)
	}
	return decoded, nil
}
