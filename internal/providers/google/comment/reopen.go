package comment

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
)

// newReopenCmd returns `comment reopen`: mark one resolved comment open
// again via a reopen action reply.
func newReopenCmd(cfg *app.Config, newSvc service.Dialer[service.CommentService]) *cobra.Command {
	return newActionCmd(cfg, newSvc, "reopen", "Reopen a resolved comment", `# Reopen a comment on a document
everything-cli google docs comment reopen 1AbCdEfGh --comment comment_1

# Reopen a comment on a presentation
everything-cli google slides comment reopen 1AbCpresentationID --comment comment_1`)
}
