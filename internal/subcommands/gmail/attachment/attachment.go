// Package attachment builds the `gmail attachment` command subtree.
package attachment

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/subcommands/gmail/service"
)

// serviceFunc builds the Gmail service a leaf's RunE uses. The gmail parent
// injects the real dialer; tests inject fakes so no leaf ever touches the
// network.
type serviceFunc func(context.Context) (service.GmailService, error)

// NewCmd returns the `gmail attachment` parent with every attachment leaf
// attached. Each leaf lives in its own file: get.go.
func NewCmd(cfg *app.Config, newSvc serviceFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "attachment",
		Short: "Manage Gmail attachments",
	}
	cmd.AddCommand(newGetCmd(cfg, newSvc))
	return cmd
}

// newAttachmentService resolves an AttachmentService from the shared gmail
// seam. The real dialer returns a service that implements every gmail
// interface; the assertion hands attachment leaves their own surface without
// growing the label one.
func newAttachmentService(ctx context.Context, newSvc serviceFunc) (service.AttachmentService, error) {
	svc, err := newSvc(ctx)
	if err != nil {
		return nil, err
	}
	attachmentSvc, ok := svc.(service.AttachmentService)
	if !ok {
		return nil, fmt.Errorf("gmail service does not implement attachment operations")
	}
	return attachmentSvc, nil
}
