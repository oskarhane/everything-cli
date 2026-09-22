package service

import (
	"context"
	"encoding/json"
	"fmt"
)

// Relation directions ListRelations reports, from the queried issue's side.
const (
	RelationOutgoing = "outgoing" // the queried issue is the relation's source
	RelationIncoming = "incoming" // the queried issue is the relation's target
)

// RelationIssue is the issue reference embedded in a Relation.
type RelationIssue struct {
	ID         string `json:"id"`
	Identifier string `json:"identifier"`
	Title      string `json:"title"`
}

// Relation is one Linear issue relation as decoded from the GraphQL API.
// Type is the wire IssueRelationType: "blocks", "duplicate", "related", or
// "similar". Linear auto-creates the inverse relation, so there is no
// "blocked-by" wire type — the blocked-by side is the same relation seen
// through issue.inverseRelations. Direction is not wire data: ListRelations
// sets it to RelationOutgoing when the relation came from issue.relations
// (the queried issue is Issue) and RelationIncoming when it came from
// issue.inverseRelations (the queried issue is RelatedIssue).
type Relation struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	Issue        *RelationIssue `json:"issue"`
	RelatedIssue *RelationIssue `json:"relatedIssue"`
	Direction    string         `json:"-"`
}

// RelationService is the relation surface the `linear issue relation`
// subtree consumes.
type RelationService interface {
	CreateRelation(ctx context.Context, issueID, relatedIssueID, relType string) (*Relation, error)
	ListRelations(ctx context.Context, issueID string) ([]Relation, error)
	DeleteRelation(ctx context.Context, relationID string) error
}

// Compile-time proof that Service satisfies the relation surface. Each
// concern file owns its own seam assertion.
var _ RelationService = (*Service)(nil)

// relationFields is the selection set relation queries and mutations return.
const relationFields = `id type issue { id identifier title } relatedIssue { id identifier title }`

// ListRelations returns every relation touching the issue issueID (UUID or
// human identifier "BLA-123") from both directions: issue.relations merged
// with issue.inverseRelations, each tagged with its Direction. Like
// Team.states these connections are bounded, not cursor-followed — one
// request covers an issue's whole relation set.
func (s *Service) ListRelations(ctx context.Context, issueID string) ([]Relation, error) {
	const query = `query($id: String!, $first: Int) {
		issue(id: $id) {
			relations(first: $first) { nodes { ` + relationFields + ` } }
			inverseRelations(first: $first) { nodes { ` + relationFields + ` } }
		}
	}`
	data, err := s.exec(ctx, query, map[string]any{"id": issueID, "first": pageSize})
	if err != nil {
		return nil, err
	}
	raw, err := dig(data, "issue")
	if err != nil {
		return nil, err
	}
	if string(raw) == "null" {
		return nil, fmt.Errorf("issue %q not found", issueID)
	}
	var payload struct {
		Relations        connection[Relation] `json:"relations"`
		InverseRelations connection[Relation] `json:"inverseRelations"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decoding issue relations: %w", err)
	}
	relations := make([]Relation, 0, len(payload.Relations.Nodes)+len(payload.InverseRelations.Nodes))
	for _, r := range payload.Relations.Nodes {
		r.Direction = RelationOutgoing
		relations = append(relations, r)
	}
	for _, r := range payload.InverseRelations.Nodes {
		r.Direction = RelationIncoming
		relations = append(relations, r)
	}
	return relations, nil
}

// CreateRelation creates a relation of type relType ("blocks", "duplicate",
// "related", "similar") from issueID to relatedIssueID (each a UUID or
// human identifier) and returns it as created. Linear auto-creates the
// inverse, so "blocked-by" is expressed by the caller as a blocks relation
// in the opposite direction, not by a type.
func (s *Service) CreateRelation(ctx context.Context, issueID, relatedIssueID, relType string) (*Relation, error) {
	const mutation = `mutation($input: IssueRelationCreateInput!) {
		issueRelationCreate(input: $input) { success issueRelation { ` + relationFields + ` } }
	}`
	input := map[string]any{"issueId": issueID, "relatedIssueId": relatedIssueID, "type": relType}
	return mutationPayload[Relation](ctx, s, mutation, map[string]any{"input": input}, "issueRelationCreate", "issueRelation")
}

// DeleteRelation deletes the relation relationID (a relation UUID, not an
// issue ID); Linear also removes the auto-created inverse. The
// issueRelationDelete payload carries success only, so this checks the flag
// directly instead of mutationPayload's node path.
func (s *Service) DeleteRelation(ctx context.Context, relationID string) error {
	const mutation = `mutation($id: String!) {
		issueRelationDelete(id: $id) { success }
	}`
	data, err := s.exec(ctx, mutation, map[string]any{"id": relationID})
	if err != nil {
		return err
	}
	raw, err := dig(data, "issueRelationDelete", "success")
	if err != nil {
		return err
	}
	var success bool
	if err := json.Unmarshal(raw, &success); err != nil {
		return fmt.Errorf("decoding issueRelationDelete success: %w", err)
	}
	if !success {
		return fmt.Errorf("linear API reported issueRelationDelete success: false")
	}
	return nil
}
