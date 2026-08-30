package message

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	gmail "google.golang.org/api/gmail/v1"

	"github.com/oskarhane/google-cli/internal/app"
)

// newMarkCmd returns `gmail message mark`: flip read/unread and starred state
// by adding or removing the UNSEEN and STARRED labels.
func newMarkCmd(cfg *app.Config, newSvc serviceFunc) *cobra.Command {
	var read, unread, starred, unstarred bool
	cmd := &cobra.Command{
		Use:   "mark <id>",
		Short: "Mark a Gmail message read/unread or starred/unstarred",
		Example: `# Mark a message as read
google-cli gmail message mark 19c2a4b7 --read

# Mark a message unread and starred
google-cli gmail message mark 19c2a4b7 --unread --starred

# Unstar a message
google-cli gmail message mark 19c2a4b7 --unstarred`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			req, err := markRequest(cmd.Flags(), read, unread, starred, unstarred)
			if err != nil {
				return err
			}
			svc, err := newMessageService(cmd.Context(), newSvc)
			if err != nil {
				return err
			}
			updated, err := svc.ModifyMessage(cmd.Context(), args[0], req)
			if err != nil {
				return err
			}
			printMessage(cmd, cfg, updated)
			return nil
		},
	}
	f := cmd.Flags()
	f.BoolVar(&read, "read", false, "Mark as read (remove UNSEEN)")
	f.BoolVar(&unread, "unread", false, "Mark as unread (add UNSEEN)")
	f.BoolVar(&starred, "starred", false, "Star the message (add STARRED)")
	f.BoolVar(&unstarred, "unstarred", false, "Remove the star (remove STARRED)")
	return cmd
}

// markRequest validates the mark flags and maps them to label changes: read
// removes UNSEEN, unread adds it; starred adds STARRED, unstarred removes it.
// At least one flag is required and each opposing pair is rejected.
func markRequest(f *pflag.FlagSet, read, unread, starred, unstarred bool) (*gmail.ModifyMessageRequest, error) {
	if read && unread {
		return nil, fmt.Errorf("--read and --unread are mutually exclusive")
	}
	if starred && unstarred {
		return nil, fmt.Errorf("--starred and --unstarred are mutually exclusive")
	}
	if !f.Changed("read") && !f.Changed("unread") && !f.Changed("starred") && !f.Changed("unstarred") {
		return nil, fmt.Errorf("nothing to mark: pass at least one of --read, --unread, --starred, --unstarred")
	}
	req := &gmail.ModifyMessageRequest{}
	if read {
		req.RemoveLabelIds = append(req.RemoveLabelIds, "UNSEEN")
	}
	if unread {
		req.AddLabelIds = append(req.AddLabelIds, "UNSEEN")
	}
	if starred {
		req.AddLabelIds = append(req.AddLabelIds, "STARRED")
	}
	if unstarred {
		req.RemoveLabelIds = append(req.RemoveLabelIds, "STARRED")
	}
	return req, nil
}
