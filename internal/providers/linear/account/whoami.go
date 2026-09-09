package account

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
)

// viewerView is the rendered shape of the viewer: the identity the account
// resolved for the invocation authenticates as, in snake_case for every
// output format.
type viewerView struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// newWhoamiCmd builds account whoami: it dials through the standard seam —
// the account resolved for the invocation (default or --account) — and asks
// Linear who that credential authenticates as, printing the viewer's id,
// name, and email. It holds no credential handling of its own; dial errors
// surface exactly like every other leaf's.
func newWhoamiCmd(cfg *app.Config, viewerDialer service.Dialer[service.ViewerService]) *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Show the identity the current Linear account authenticates as",
		Example: `# Show who the default account is as JSON
everything-cli linear account whoami --format json

# Show who the "work" account is as a table
everything-cli linear account whoami --account work --format table`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, err := viewerDialer(cmd.Context())
			if err != nil {
				return err
			}
			viewer, err := svc.GetViewer(cmd.Context())
			if err != nil {
				return err
			}
			view := viewerView{ID: viewer.ID, Name: viewer.Name, Email: viewer.Email}
			output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format),
				[]string{"id", "name", "email"}, view,
				[]map[string]any{{"id": view.ID, "name": view.Name, "email": view.Email}})
			return nil
		},
	}
}
