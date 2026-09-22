package relation

import (
	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
)

// View is the rendered shape of one relation, normalized around the issue
// the caller asked about: Type and Direction read from that issue's
// perspective, and Identifier/Title name the other issue. Output field
// names are snake_case per the casing rule.
type View struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Direction  string `json:"direction"`
	Identifier string `json:"identifier"`
	Title      string `json:"title"`
}

// Fields are the table columns of a relation.
var Fields = []string{"id", "type", "direction", "identifier", "title"}

// toView maps a wire relation to its rendered shape from the perspective of
// the issue whose relations were listed (or the --issue of a create): the
// other issue is RelatedIssue on outgoing relations and Issue on incoming.
func toView(r *service.Relation) View {
	v := View{ID: r.ID, Type: perspectiveType(r.Type, r.Direction), Direction: r.Direction}
	other := r.RelatedIssue
	if r.Direction == service.RelationIncoming {
		other = r.Issue
	}
	if other != nil {
		v.Identifier = other.Identifier
		v.Title = other.Title
	}
	return v
}

// perspectiveType renders a wire relation type from the queried issue's
// side: blocks seen from the incoming side is blocked-by and duplicate
// reads as duplicates; related and similar are symmetric.
func perspectiveType(wireType, direction string) string {
	switch {
	case wireType == typeBlocks && direction == service.RelationIncoming:
		return typeBlockedBy
	case wireType == "duplicate":
		return typeDuplicates
	default:
		return wireType
	}
}

// row renders a relation view as a full JSON/TOON and table row: the view
// is flat, so one shape serves every format.
func row(v View) map[string]any {
	return map[string]any{
		"id":         v.ID,
		"type":       v.Type,
		"direction":  v.Direction,
		"identifier": v.Identifier,
		"title":      v.Title,
	}
}
