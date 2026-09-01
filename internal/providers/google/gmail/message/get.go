package message

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
	"github.com/oskarhane/everything-cli/internal/providers/google/gmail/service"
)

// newGetCmd returns `gmail message get`: one message by id. The default view
// shows the parsed payload headers; --raw prints the decoded RFC 2822 message
// as plain text, ignoring --format. Control bytes (ANSI/OSC escapes, BEL,
// other C0 + DEL) are sanitised to "?" so a hostile email cannot spoof the
// terminal or trigger OSC 52 clipboard writes; "\t", "\n", "\r" survive.
func newGetCmd(cfg *app.Config, newSvc service.Dialer[service.MessageService]) *cobra.Command {
	var raw bool
	cmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Show a Gmail message by id",
		Example: `# Show a message with its From/Subject/Date headers as JSON
everything-cli google gmail message get 19c2a4b7 --format json

# Print the raw RFC 2822 message as plain text
everything-cli google gmail message get 19c2a4b7 --raw

# Show the same message as a table
everything-cli google gmail message get 19c2a4b7 --format table`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc(cmd.Context())
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
				// Security (S4): the raw path prints the RFC 2822 source
				// verbatim, bypassing output.Print, so a malicious email could
				// inject ANSI/OSC escape sequences (e.g. OSC 52 clipboard
				// overwrite). A single StripControl pass over the whole
				// decoded-raw string is sufficient: the raw field is the
				// *undecoded* transfer-encoded source (base64 and
				// quoted-printable bodies stay base64/QP here), so escapes can
				// only appear as literal bytes in the decoded text — no nested
				// decode can hide them. "\t", "\n", "\r" survive.
				_, err = fmt.Fprintln(cmd.OutOrStdout(), output.StripControl(decoded))
				if err != nil {
					return err
				}
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
