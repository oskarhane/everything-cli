package attachment

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
)

// newCreateCmd returns `linear issue attachment create`: link a URL to an
// issue. Re-posting the same url to the same issue updates the existing
// attachment rather than duplicating it, so the command is idempotent by
// design.
func newCreateCmd(cfg *app.Config, newSvc service.Dialer[service.AttachmentService]) *cobra.Command {
	var (
		url      string
		title    string
		subtitle string
	)
	cmd := &cobra.Command{
		Use:   "create <id>",
		Short: "Link an attachment to a Linear issue",
		Example: `# Link an attachment to an issue
everything-cli linear issue attachment create ENG-123 --url https://example.com/rfc --title "RFC"

# Include a subtitle
everything-cli linear issue attachment create ENG-123 --url https://example.com/rfc \
  --title "RFC" --subtitle "Design doc"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			created, err := svc.CreateAttachment(cmd.Context(), args[0], service.CreateAttachmentInput{
				URL:      url,
				Title:    title,
				Subtitle: subtitle,
			})
			if err != nil {
				return err
			}
			printAttachment(cmd, cfg, created)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&url, "url", "", "URL to link (required)")
	f.StringVar(&title, "title", "", "Attachment title (required)")
	f.StringVar(&subtitle, "subtitle", "", "Attachment subtitle")
	_ = cmd.MarkFlagRequired("url")
	_ = cmd.MarkFlagRequired("title")
	return cmd
}
