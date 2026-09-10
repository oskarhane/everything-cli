package comment

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
)

// newResolveCmd returns `comment resolve`: mark one comment resolved via a
// resolve action reply.
func newResolveCmd(cfg *app.Config, newSvc service.Dialer[service.CommentService]) *cobra.Command {
	return newActionCmd(cfg, newSvc, "resolve", "Mark a comment resolved", `# Resolve a comment on a document
everything-cli google docs comment resolve 1AbCdEfGh --comment comment_1

# Resolve a comment on a presentation
everything-cli google slides comment resolve 1AbCpresentationID --comment comment_1`)
}
