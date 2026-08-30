// Package thread builds the `gmail thread` command subtree.
package thread

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

// NewCmd returns the `gmail thread` parent with every thread leaf attached.
// Each leaf lives in its own file: list.go, get.go.
func NewCmd(cfg *app.Config, newSvc serviceFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "thread",
		Short: "Manage Gmail threads",
	}
	cmd.AddCommand(newListCmd(cfg, newSvc))
	cmd.AddCommand(newGetCmd(cfg, newSvc))
	return cmd
}

// newThreadService resolves a ThreadService from the shared gmail seam. The
// real dialer returns a service that implements every gmail interface; the
// assertion hands thread leaves their own surface without growing the label
// one.
func newThreadService(ctx context.Context, newSvc serviceFunc) (service.ThreadService, error) {
	svc, err := newSvc(ctx)
	if err != nil {
		return nil, err
	}
	threadSvc, ok := svc.(service.ThreadService)
	if !ok {
		return nil, fmt.Errorf("gmail service does not implement thread operations")
	}
	return threadSvc, nil
}
