package attachment

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
)

// newListCmd returns `linear issue attachment list`: every link attachment
// on one issue by UUID or human identifier ("BLA-123").
func newListCmd(cfg *app.Config, newSvc service.Dialer[service.AttachmentService]) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list <id>",
		Short: "List attachments on a Linear issue",
		Example: `# List an issue's attachments as JSON
everything-cli linear issue attachment list BLA-123 --format json

# List the same issue's attachments as a table
everything-cli linear issue attachment list 8c8a1b2c-0000-4000-8000-000000000001 --format table`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			attachments, err := svc.ListAttachments(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			printAttachmentList(cmd, cfg, attachments)
			return nil
		},
	}
	return cmd
}

// printAttachmentList renders an attachment list under the one-row-vs-array
// output convention: zero attachments print as an empty list, one as a
// single object.
func printAttachmentList(cmd *cobra.Command, cfg *app.Config, attachments []service.Attachment) {
	jsonRows := make([]map[string]any, 0, len(attachments))
	tableRows := make([]map[string]any, 0, len(attachments))
	for i := range attachments {
		v := toAttachmentView(&attachments[i])
		jsonRows = append(jsonRows, attachmentJSONRow(v))
		tableRows = append(tableRows, attachmentTableRow(v))
	}
	output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format), tableFields, jsonRows, tableRows)
}
