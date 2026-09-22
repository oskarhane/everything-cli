package relation

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
)

// newDeleteCmd returns `linear issue relation delete`: delete one relation
// by its relation UUID (the id field of relation list output), not an issue
// ID. Linear also removes the auto-created inverse relation.
func newDeleteCmd(_ *app.Config, newSvc service.Dialer[service.RelationService]) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <relation-id>",
		Short: "Delete a relation between two Linear issues",
		Example: `# Delete the relation whose id appears in relation list output
everything-cli linear issue relation delete 3f4a2b1c-0000-4000-8000-0000000000aa

# Find a relation's id before deleting it
everything-cli linear issue relation list --issue BLA-123`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			return svc.DeleteRelation(cmd.Context(), args[0])
		},
	}
}
