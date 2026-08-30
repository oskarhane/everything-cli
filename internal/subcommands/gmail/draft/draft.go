// Package draft builds the `gmail draft` command subtree.
package draft

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

// NewCmd returns the `gmail draft` parent with every draft leaf attached.
// Each leaf lives in its own file: list.go, get.go, create.go, send.go,
// delete.go.
func NewCmd(cfg *app.Config, newSvc serviceFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "draft",
		Short: "Manage Gmail drafts",
	}
	cmd.AddCommand(newListCmd(cfg, newSvc))
	cmd.AddCommand(newGetCmd(cfg, newSvc))
	cmd.AddCommand(newCreateCmd(cfg, newSvc))
	cmd.AddCommand(newSendCmd(cfg, newSvc))
	cmd.AddCommand(newDeleteCmd(cfg, newSvc))
	return cmd
}

// newDraftService resolves a DraftService from the shared gmail seam. The
// real dialer returns a service that implements every gmail interface; the
// assertion hands draft leaves their own surface without growing the label
// one.
func newDraftService(ctx context.Context, newSvc serviceFunc) (service.DraftService, error) {
	svc, err := newSvc(ctx)
	if err != nil {
		return nil, err
	}
	draftSvc, ok := svc.(service.DraftService)
	if !ok {
		return nil, fmt.Errorf("gmail service does not implement draft operations")
	}
	return draftSvc, nil
}
