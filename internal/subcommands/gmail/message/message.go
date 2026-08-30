// Package message builds the `gmail message` command subtree.
package message

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

// NewCmd returns the `gmail message` parent with every message leaf attached.
// Each leaf lives in its own file: list.go, get.go, send.go, trash.go,
// untrash.go, delete.go, mark.go, modify.go.
func NewCmd(cfg *app.Config, newSvc serviceFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "message",
		Short: "Manage Gmail messages",
	}
	cmd.AddCommand(newListCmd(cfg, newSvc))
	cmd.AddCommand(newGetCmd(cfg, newSvc))
	cmd.AddCommand(newSendCmd(cfg, newSvc))
	cmd.AddCommand(newTrashCmd(cfg, newSvc))
	cmd.AddCommand(newUntrashCmd(cfg, newSvc))
	cmd.AddCommand(newDeleteCmd(cfg, newSvc))
	cmd.AddCommand(newMarkCmd(cfg, newSvc))
	cmd.AddCommand(newModifyCmd(cfg, newSvc))
	return cmd
}

// newMessageService resolves a MessageService from the shared gmail seam. The
// real dialer returns a service that implements both the label and message
// interfaces; the assertion hands message leaves their own surface without
// growing the label one.
func newMessageService(ctx context.Context, newSvc serviceFunc) (service.MessageService, error) {
	svc, err := newSvc(ctx)
	if err != nil {
		return nil, err
	}
	messageSvc, ok := svc.(service.MessageService)
	if !ok {
		return nil, fmt.Errorf("gmail service does not implement message operations")
	}
	return messageSvc, nil
}
