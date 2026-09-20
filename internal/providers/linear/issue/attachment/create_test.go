package attachment

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestCreateRequiresURLAndTitle(t *testing.T) {
	svc := &fakeService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newCreateCmd, svc, "json"), "ENG-1")
	require.Error(t, err)

	_, err = cmdtest.RunCmdErr(t, newLeafCmd(newCreateCmd, svc, "json"),
		"ENG-1", "--url", "https://example.com/rfc")
	require.Error(t, err, "--title is required")

	_, err = cmdtest.RunCmdErr(t, newLeafCmd(newCreateCmd, svc, "json"),
		"ENG-1", "--title", "RFC")
	require.Error(t, err, "--url is required")
}

func TestCreatePassesIDAndFlags(t *testing.T) {
	svc := &fakeService{}
	out := cmdtest.RunCmd(t, newLeafCmd(newCreateCmd, svc, "json"),
		"ENG-1", "--url", "https://example.com/rfc", "--title", "RFC", "--subtitle", "Design doc")

	require.Equal(t, "ENG-1", svc.gotID)
	require.Equal(t, "https://example.com/rfc", svc.created.URL)
	require.Equal(t, "RFC", svc.created.Title)
	require.Equal(t, "Design doc", svc.created.Subtitle)

	m, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok, "one attachment renders as a single object: %v", out)
	require.Equal(t, "attachment_1", m["id"])
	require.Equal(t, "RFC", m["title"])
	require.Equal(t, "https://example.com/rfc", m["url"])
	require.Equal(t, "Design doc", m["subtitle"])
	require.Contains(t, m, "created_at")
	require.NotContains(t, m, "createdAt")
}

func TestCreateJSONOmitsEmptySubtitle(t *testing.T) {
	svc := &fakeService{}
	out := cmdtest.RunCmd(t, newLeafCmd(newCreateCmd, svc, "json"),
		"ENG-1", "--url", "https://example.com/rfc", "--title", "RFC")

	m, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok)
	require.NotContains(t, m, "subtitle")
}

func TestCreateTable(t *testing.T) {
	svc := &fakeService{}
	out := cmdtest.RunCmd(t, newLeafCmd(newCreateCmd, svc, "table"),
		"ENG-1", "--url", "https://example.com/rfc", "--title", "RFC")

	// go-pretty StyleLight upper-cases header cells.
	require.Contains(t, out, "CREATED_AT")
	require.Contains(t, out, "TITLE")
	require.Contains(t, out, "URL")
	require.Contains(t, out, "RFC")
	require.Contains(t, out, "https://example.com/rfc")
}

func TestCreateToon(t *testing.T) {
	svc := &fakeService{}
	out := cmdtest.RunCmd(t, newLeafCmd(newCreateCmd, svc, "toon"),
		"ENG-1", "--url", "https://example.com/rfc", "--title", "RFC")

	require.Contains(t, out, "id: attachment_1")
	require.Contains(t, out, "title: RFC")
	// toon quotes values containing characters like `:` and `.`.
	require.Contains(t, out, `created_at: "2026-08-01T10:00:00.000Z"`)
}

func TestCreatePropagatesServiceError(t *testing.T) {
	svc := &fakeService{err: errors.New("boom")}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newCreateCmd, svc, "json"),
		"ENG-1", "--url", "https://example.com/rfc", "--title", "RFC")

	require.ErrorContains(t, err, "boom")
	require.Equal(t, "ENG-1", svc.gotID)
}
