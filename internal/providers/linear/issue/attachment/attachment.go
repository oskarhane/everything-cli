// Package attachment builds the `linear issue attachment` command tree. It is
// a separate package from `issue` so `issue.NewCmd` can attach it without an
// import cycle; the local view types keep that boundary intact.
package attachment

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
)

// NewCmd returns the `linear issue attachment` parent command with its leaves
// attached. Every leaf lives in its own file with one AddCommand line per
// leaf.
func NewCmd(cfg *app.Config, newSvc service.Dialer[service.AttachmentService]) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "attachment",
		Short: "Manage Linear issue attachments",
	}
	cmd.AddCommand(newCreateCmd(cfg, newSvc))
	return cmd
}

// attachmentView is the rendered shape of a link attachment: output field
// names are snake_case per the casing rule.
type attachmentView struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	URL       string `json:"url"`
	Subtitle  string `json:"subtitle,omitempty"`
	CreatedAt string `json:"created_at"`
}

// createFields are the table columns of attachment create.
var createFields = []string{"created_at", "title", "url"}

// toAttachmentView maps the wire attachment to its rendered shape.
func toAttachmentView(a *service.Attachment) attachmentView {
	return attachmentView{
		ID:        a.ID,
		Title:     a.Title,
		URL:       a.URL,
		Subtitle:  a.Subtitle,
		CreatedAt: a.CreatedAt,
	}
}

// attachmentTableRow flattens a view into table-row cells.
func attachmentTableRow(v attachmentView) map[string]any {
	return map[string]any{
		"created_at": v.CreatedAt,
		"title":      v.Title,
		"url":        v.URL,
	}
}

// printAttachment renders one attachment as a single object under the
// one-row-vs-array output convention.
func printAttachment(cmd *cobra.Command, cfg *app.Config, a *service.Attachment) {
	v := toAttachmentView(a)
	output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format), createFields, v,
		[]map[string]any{attachmentTableRow(v)})
}
