// Package gmail builds the `gmail` command tree.
package gmail

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/subcommands/gmail/attachment"
	"github.com/oskarhane/google-cli/internal/subcommands/gmail/draft"
	"github.com/oskarhane/google-cli/internal/subcommands/gmail/label"
	"github.com/oskarhane/google-cli/internal/subcommands/gmail/message"
	"github.com/oskarhane/google-cli/internal/subcommands/gmail/service"
	"github.com/oskarhane/google-cli/internal/subcommands/gmail/thread"
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
	// The concrete service implements every gmail interface; As narrows the
	// shared seam to each subtree's own surface.
	cmd.AddCommand(label.NewCmd(cfg, newSvc))
	cmd.AddCommand(message.NewCmd(cfg, func(ctx context.Context) (service.MessageService, error) {
		return service.As[service.MessageService](newSvc(ctx))
	}))
	cmd.AddCommand(thread.NewCmd(cfg, func(ctx context.Context) (service.ThreadService, error) {
		return service.As[service.ThreadService](newSvc(ctx))
	}))
	cmd.AddCommand(draft.NewCmd(cfg, func(ctx context.Context) (service.DraftService, error) {
		return service.As[service.DraftService](newSvc(ctx))
	}))
	cmd.AddCommand(attachment.NewCmd(cfg, func(ctx context.Context) (service.AttachmentService, error) {
		return service.As[service.AttachmentService](newSvc(ctx))
	}))
	return cmd
}
