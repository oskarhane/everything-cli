// Package gmail builds the `gmail` command tree.
package gmail

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/subcommands/gmail/label"
	"github.com/oskarhane/google-cli/internal/subcommands/gmail/service"
)

// NewCmd returns the `gmail` parent command with its subtrees attached.
// Leaves live in their own files under label/ (and later sibling dirs).
func NewCmd(cfg *app.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gmail",
		Short: "Manage Gmail labels, messages, drafts, threads, and attachments",
	}
	newSvc := func(ctx context.Context) (service.GmailService, error) {
		return dial(ctx, cfg)
	}
	cmd.AddCommand(label.NewCmd(cfg, newSvc))
	// Later nodes attach the remaining subtrees here:
	//   cmd.AddCommand(message.NewCmd(cfg, newSvc))
	//   cmd.AddCommand(thread.NewCmd(cfg, newSvc))
	//   cmd.AddCommand(draft.NewCmd(cfg, newSvc))
	//   cmd.AddCommand(attachment.NewCmd(cfg, newSvc))
	return cmd
}
