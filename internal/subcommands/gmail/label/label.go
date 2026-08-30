// Package label builds the `gmail label` command subtree.
package label

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/subcommands/gmail/service"
)

// serviceFunc builds the Gmail service a leaf's RunE uses. The gmail parent
// injects the real dialer; tests inject fakes so no leaf ever touches the
// network.
type serviceFunc func(context.Context) (service.GmailService, error)

// NewCmd returns the `gmail label` parent with every label leaf attached.
// Each leaf lives in its own file: list.go, get.go, create.go, update.go,
// delete.go.
func NewCmd(cfg *app.Config, newSvc serviceFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "label",
		Short: "Manage Gmail labels",
	}
	cmd.AddCommand(newListCmd(cfg, newSvc))
	cmd.AddCommand(newGetCmd(cfg, newSvc))
	cmd.AddCommand(newCreateCmd(cfg, newSvc))
	cmd.AddCommand(newUpdateCmd(cfg, newSvc))
	cmd.AddCommand(newDeleteCmd(cfg, newSvc))
	return cmd
}
