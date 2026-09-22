package relation

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
)

// newCreateCmd returns `linear issue relation create`: relate --issue to
// --related. --type is the CLI-facing type; blocked-by inverts into a wire
// blocks relation from --related to --issue. An unknown --type fails before
// the service is dialed.
func newCreateCmd(cfg *app.Config, newSvc service.Dialer[service.RelationService]) *cobra.Command {
	var (
		issue   string
		related string
		relType string
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a relation between two Linear issues",
		Example: `# BLA-123 blocks BLA-456
everything-cli linear issue relation create --issue BLA-123 --related BLA-456 --type blocks

# BLA-123 is blocked by BLA-456 (a blocks relation from BLA-456 to BLA-123)
everything-cli linear issue relation create --issue BLA-123 --related BLA-456 --type blocked-by

# Mark BLA-123 as a duplicate of BLA-456
everything-cli linear issue relation create --issue BLA-123 --related BLA-456 --type duplicates`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			plan, err := resolveCreate(issue, related, relType)
			if err != nil {
				return err
			}
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			rel, err := svc.CreateRelation(cmd.Context(), plan.issueID, plan.relatedID, plan.wireType)
			if err != nil {
				return err
			}
			// The wire relation carries no direction; the echo reads from
			// --issue's perspective, which resolveCreate already computed.
			rel.Direction = plan.direction
			printRelation(cmd, cfg, rel)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&issue, "issue", "", "Issue UUID or identifier the relation reads from (required)")
	f.StringVar(&related, "related", "", "The other issue's UUID or identifier (required)")
	f.StringVar(&relType, "type", "", "Relation type: blocks, blocked-by, duplicates, or related (required)")
	_ = cmd.MarkFlagRequired("issue")
	_ = cmd.MarkFlagRequired("related")
	_ = cmd.MarkFlagRequired("type")
	return cmd
}

// createPlan is the resolved wire call for a create: the source and target
// issue, the wire relation type, and the direction the created relation
// reads from --issue's perspective.
type createPlan struct {
	issueID   string
	relatedID string
	wireType  string
	direction string
}

// resolveCreate maps the create flags to a createPlan. blocked-by inverts:
// the related issue blocks --issue, so --related is the wire source.
func resolveCreate(issue, related, relType string) (createPlan, error) {
	switch relType {
	case typeBlocks:
		return createPlan{issue, related, typeBlocks, service.RelationOutgoing}, nil
	case typeBlockedBy:
		return createPlan{related, issue, typeBlocks, service.RelationIncoming}, nil
	case typeDuplicates:
		return createPlan{issue, related, "duplicate", service.RelationOutgoing}, nil
	case typeRelated:
		return createPlan{issue, related, typeRelated, service.RelationOutgoing}, nil
	}
	return createPlan{}, fmt.Errorf("invalid --type %q: want one of blocks, blocked-by, duplicates, related", relType)
}

// printRelation renders one created relation as a single object under the
// one-row-vs-array output convention.
func printRelation(cmd *cobra.Command, cfg *app.Config, r *service.Relation) {
	v := toView(r)
	output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format), Fields, v,
		[]map[string]any{row(v)})
}
