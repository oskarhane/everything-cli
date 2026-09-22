package relation

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestCreateRequiresAllFlags(t *testing.T) {
	svc := &fakeService{created: &seedRelations()[0]}
	_, err := cmdtest.RunCmdErr(t, createCmd(svc, "json"), "--issue", "ENG-1", "--related", "ENG-2")

	require.Error(t, err, "cobra rejects a missing --type before RunE")
	require.False(t, svc.dialed, "the service is never dialed without every flag")
}

func TestCreateInvalidTypeRejectedBeforeDial(t *testing.T) {
	svc := &fakeService{created: &seedRelations()[0]}
	_, err := cmdtest.RunCmdErr(t, createCmd(svc, "json"),
		"--issue", "ENG-1", "--related", "ENG-2", "--type", "depends-on")

	require.ErrorContains(t, err, `invalid --type "depends-on"`)
	require.ErrorContains(t, err, "blocks, blocked-by, duplicates, related")
	require.False(t, svc.dialed, "an unknown type never reaches the API")
}

func TestCreateBlocksPassesThrough(t *testing.T) {
	svc := &fakeService{created: &seedRelations()[0]}
	out := cmdtest.RunCmd(t, createCmd(svc, "json"),
		"--issue", "ENG-1", "--related", "ENG-2", "--type", "blocks")

	require.Equal(t, "ENG-1", svc.createIssueID)
	require.Equal(t, "ENG-2", svc.createRelated)
	require.Equal(t, "blocks", svc.createType)

	m, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok, "one created relation renders as a single object: %v", out)
	require.Equal(t, "rel_1", m["id"])
	require.Equal(t, "blocks", m["type"])
	require.Equal(t, "outgoing", m["direction"])
	require.Equal(t, "ENG-2", m["identifier"])
	require.Equal(t, "Second", m["title"])
	cmdtest.RequireSnakeCase(t, cmdtest.JSONKeys(t, m))
}

func TestCreateBlockedByInvertsDirection(t *testing.T) {
	svc := &fakeService{created: &seedRelations()[1]}
	out := cmdtest.RunCmd(t, createCmd(svc, "json"),
		"--issue", "ENG-1", "--related", "ENG-3", "--type", "blocked-by")

	// blocked-by inverts: ENG-3 blocks ENG-1, so --related is the wire
	// source and the wire type is blocks.
	require.Equal(t, "ENG-3", svc.createIssueID, "--related becomes the wire source")
	require.Equal(t, "ENG-1", svc.createRelated, "--issue becomes the wire target")
	require.Equal(t, "blocks", svc.createType, "blocked-by is a wire blocks relation")

	// The echo still reads from --issue's perspective.
	m, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok)
	require.Equal(t, "blocked-by", m["type"])
	require.Equal(t, "incoming", m["direction"])
	require.Equal(t, "ENG-3", m["identifier"], "the other issue is the blocker")
	require.Equal(t, "Third", m["title"])
}

func TestCreateDuplicatesMapsToDuplicate(t *testing.T) {
	created := seedRelations()[0]
	created.Type = "duplicate"
	svc := &fakeService{created: &created}
	out := cmdtest.RunCmd(t, createCmd(svc, "json"),
		"--issue", "ENG-1", "--related", "ENG-2", "--type", "duplicates")

	require.Equal(t, "duplicate", svc.createType, "duplicates maps to the wire duplicate type")
	require.Equal(t, "ENG-1", svc.createIssueID)
	require.Equal(t, "ENG-2", svc.createRelated)

	m, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok)
	require.Equal(t, "duplicates", m["type"])
	require.Equal(t, "outgoing", m["direction"])
}

func TestCreateRelatedPassesThrough(t *testing.T) {
	created := seedRelations()[0]
	created.Type = "related"
	svc := &fakeService{created: &created}
	out := cmdtest.RunCmd(t, createCmd(svc, "json"),
		"--issue", "ENG-1", "--related", "ENG-2", "--type", "related")

	require.Equal(t, "related", svc.createType)
	m, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok)
	require.Equal(t, "related", m["type"])
}

func TestCreateTable(t *testing.T) {
	svc := &fakeService{created: &seedRelations()[0]}
	out := cmdtest.RunCmd(t, createCmd(svc, "table"),
		"--issue", "ENG-1", "--related", "ENG-2", "--type", "blocks")

	// go-pretty StyleLight upper-cases header cells.
	require.Contains(t, out, "TYPE")
	require.Contains(t, out, "DIRECTION")
	require.Contains(t, out, "IDENTIFIER")
	require.Contains(t, out, "TITLE")
	require.Contains(t, out, "blocks")
	require.Contains(t, out, "outgoing")
	require.Contains(t, out, "ENG-2")
}

func TestCreateToon(t *testing.T) {
	svc := &fakeService{created: &seedRelations()[0]}
	out := cmdtest.RunCmd(t, createCmd(svc, "toon"),
		"--issue", "ENG-1", "--related", "ENG-2", "--type", "blocks")

	require.Contains(t, out, "type:")
	require.Contains(t, out, "blocks")
}

func TestCreateError(t *testing.T) {
	svc := &fakeService{err: errors.New(`issue "ENG-999" not found`)}
	_, err := cmdtest.RunCmdErr(t, createCmd(svc, "json"),
		"--issue", "ENG-999", "--related", "ENG-2", "--type", "related")

	require.ErrorContains(t, err, `issue "ENG-999" not found`)
}

func TestCreateNilOtherIssueRendersEmpty(t *testing.T) {
	created := &service.Relation{ID: "rel_9", Type: "related"}
	svc := &fakeService{created: created}
	out := cmdtest.RunCmd(t, createCmd(svc, "json"),
		"--issue", "ENG-1", "--related", "ENG-2", "--type", "related")

	m, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok)
	require.Equal(t, "", m["identifier"], "a nil related-issue ref decodes to empty fields")
	require.Equal(t, "", m["title"])
}
