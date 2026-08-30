package thread

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/subcommands/gmail/service"
)

// newGetCmd returns `gmail thread get`: one thread by id, showing its
// messages with their From/Subject/Date headers and snippets.
func newGetCmd(cfg *app.Config, newSvc service.Dialer[service.ThreadService]) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Show a Gmail thread by id",
		Example: `# Show a thread's messages with their headers as JSON
google-cli gmail thread get thread_19c2a4b7 --format json

# Show the same thread as a table
google-cli gmail thread get thread_19c2a4b7 --format table`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			thread, err := svc.GetThread(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			printThreadMessages(cmd, cfg, thread)
			return nil
		},
	}
	return cmd
}
