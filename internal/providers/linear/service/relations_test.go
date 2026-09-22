package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// relationNode returns a mock issue-relation node between two issues.
func relationNode(id, relType, issueID, issueIdentifier, relatedID, relatedIdentifier string) map[string]any {
	return map[string]any{
		"id":   id,
		"type": relType,
		"issue": map[string]any{
			"id": issueID, "identifier": issueIdentifier, "title": issueIdentifier + " title",
		},
		"relatedIssue": map[string]any{
			"id": relatedID, "identifier": relatedIdentifier, "title": relatedIdentifier + " title",
		},
	}
}

func TestListRelationsMergesBothDirections(t *testing.T) {
	srv, calls := mockGraphQL(t, func(gqlCall) any {
		return map[string]any{"issue": map[string]any{
			"relations": conn([]any{
				relationNode("rel_1", "blocks", "issue_1", "ENG-1", "issue_2", "ENG-2"),
			}, false, ""),
			"inverseRelations": conn([]any{
				relationNode("rel_2", "blocks", "issue_3", "ENG-3", "issue_1", "ENG-1"),
			}, false, ""),
		}}
	})
	svc := newTestService(srv)

	relations, err := svc.ListRelations(context.Background(), "ENG-1")
	require.NoError(t, err)
	require.Len(t, relations, 2, "outgoing and incoming relations merge into one list")

	out := relations[0]
	require.Equal(t, "rel_1", out.ID)
	require.Equal(t, "blocks", out.Type)
	require.Equal(t, RelationOutgoing, out.Direction)
	require.Equal(t, "ENG-1", out.Issue.Identifier)
	require.Equal(t, "ENG-2", out.RelatedIssue.Identifier)
	require.Equal(t, "ENG-2 title", out.RelatedIssue.Title)

	in := relations[1]
	require.Equal(t, "rel_2", in.ID)
	require.Equal(t, RelationIncoming, in.Direction, "inverse relations are tagged incoming")
	require.Equal(t, "ENG-3", in.Issue.Identifier)
	require.Equal(t, "ENG-1", in.RelatedIssue.Identifier)

	require.Len(t, *calls, 1, "both directions come from one request")
	require.Equal(t, "ENG-1", (*calls)[0].Variables["id"])
	require.Equal(t, float64(pageSize), (*calls)[0].Variables["first"])
	require.Contains(t, (*calls)[0].Query, "relations(first: $first)")
	require.Contains(t, (*calls)[0].Query, "inverseRelations(first: $first)")
}

func TestListRelationsIssueNotFound(t *testing.T) {
	srv, _ := mockGraphQL(t, func(gqlCall) any {
		return map[string]any{"issue": nil}
	})
	svc := newTestService(srv)

	_, err := svc.ListRelations(context.Background(), "ENG-999")
	require.ErrorContains(t, err, `issue "ENG-999" not found`)
}

func TestCreateRelationSendsInputAndDecodesRelation(t *testing.T) {
	srv, calls := mockGraphQL(t, func(gqlCall) any {
		return map[string]any{"issueRelationCreate": map[string]any{
			"success":       true,
			"issueRelation": relationNode("rel_new", "blocks", "issue_1", "ENG-1", "issue_2", "ENG-2"),
		}}
	})
	svc := newTestService(srv)

	rel, err := svc.CreateRelation(context.Background(), "ENG-1", "ENG-2", "blocks")
	require.NoError(t, err)
	require.Equal(t, "rel_new", rel.ID)
	require.Equal(t, "blocks", rel.Type)
	require.Equal(t, "ENG-1", rel.Issue.Identifier)
	require.Equal(t, "ENG-2", rel.RelatedIssue.Identifier)

	require.Len(t, *calls, 1)
	require.Contains(t, (*calls)[0].Query, "issueRelationCreate(input: $input)")
	require.Equal(t, map[string]any{
		"input": map[string]any{
			"issueId":        "ENG-1",
			"relatedIssueId": "ENG-2",
			"type":           "blocks",
		},
	}, (*calls)[0].Variables)
}

func TestCreateRelationSuccessFalseNamesIssueRelationCreate(t *testing.T) {
	srv, _ := mockGraphQL(t, func(gqlCall) any {
		return map[string]any{"issueRelationCreate": map[string]any{"success": false, "issueRelation": nil}}
	})
	svc := newTestService(srv)

	_, err := svc.CreateRelation(context.Background(), "ENG-1", "ENG-2", "related")
	require.ErrorContains(t, err, "issueRelationCreate")
	require.ErrorContains(t, err, "success: false")
}

func TestDeleteRelationSendsID(t *testing.T) {
	srv, calls := mockGraphQL(t, func(gqlCall) any {
		return map[string]any{"issueRelationDelete": map[string]any{"success": true}}
	})
	svc := newTestService(srv)

	err := svc.DeleteRelation(context.Background(), "rel_1")
	require.NoError(t, err)

	require.Len(t, *calls, 1)
	require.Contains(t, (*calls)[0].Query, "issueRelationDelete(id: $id)")
	require.Equal(t, "rel_1", (*calls)[0].Variables["id"])
}

func TestDeleteRelationSuccessFalseSurfaces(t *testing.T) {
	srv, _ := mockGraphQL(t, func(gqlCall) any {
		return map[string]any{"issueRelationDelete": map[string]any{"success": false}}
	})
	svc := newTestService(srv)

	err := svc.DeleteRelation(context.Background(), "rel_1")
	require.ErrorContains(t, err, "issueRelationDelete")
	require.ErrorContains(t, err, "success: false")
}
