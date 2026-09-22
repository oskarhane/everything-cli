// Package relation builds the `linear issue relation` command tree: create,
// list, and delete for relations between issues. Linear has no "blocked-by"
// wire type — it is the inverse of blocks — so the package owns the CLI-type
// to wire-type mapping and the direction-aware rendering shared by the
// create echo and the list leaf.
package relation

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
)

// CLI relation types accepted by create's --type. blocked-by is expressed
// on the wire as a blocks relation in the opposite direction; duplicates
// maps to the wire duplicate type.
const (
	typeBlocks     = "blocks"
	typeBlockedBy  = "blocked-by"
	typeDuplicates = "duplicates"
	typeRelated    = "related"
)

// NewCmd returns the `relation` parent with its leaves attached. Every leaf
// lives in its own file with one AddCommand line per leaf.
func NewCmd(cfg *app.Config, newSvc service.Dialer[service.RelationService]) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "relation",
		Short: "Manage relations between Linear issues",
	}
	cmd.AddCommand(newCreateCmd(cfg, newSvc))
	cmd.AddCommand(newListCmd(cfg, newSvc))
	cmd.AddCommand(newDeleteCmd(cfg, newSvc))
	return cmd
}
