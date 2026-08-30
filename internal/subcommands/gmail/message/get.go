package message

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
)

// newGetCmd returns `gmail message get`: one message by id. The default view
// shows the parsed payload headers; --raw prints the decoded RFC 2822 message
// as plain text, ignoring --format.
func newGetCmd(cfg *app.Config, newSvc serviceFunc) *cobra.Command {
	var raw bool
	cmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Show a Gmail message by id",
		Example: `# Show a message with its From/Subject/Date headers as JSON
google-cli gmail message get 19c2a4b7 --format json

# Print the raw RFC 2822 message as plain text
google-cli gmail message get 19c2a4b7 --raw

# Show the same message as a table
google-cli gmail message get 19c2a4b7 --format table`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newMessageService(cmd.Context(), newSvc)
			if err != nil {
				return err
			}
			format := "full"
			if raw {
				format = "raw"
			}
			message, err := svc.GetMessage(cmd.Context(), args[0], format)
			if err != nil {
				return err
			}
			if raw {
				decoded, err := decodeRaw(message.Raw)
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), decoded)
				return nil
			}
			printMessageDetail(cmd, cfg, message)
			return nil
		},
	}
	cmd.Flags().BoolVar(&raw, "raw", false, "Print the decoded raw RFC 2822 message as plain text")
	return cmd
}

// decodeRaw decodes the API's base64url raw message field (unpadded) for
// display.
func decodeRaw(raw string) (string, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(raw, "="))
	if err != nil {
		return "", fmt.Errorf("decoding raw message: %w", err)
	}
	return string(decoded), nil
}
