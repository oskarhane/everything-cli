package draft

import (
	"errors"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestCreatePlainMIME(t *testing.T) {
	svc := &fakeService{}
	runCmd(t, newLeafCmd(newCreateCmd, svc, "json"),
		"--to", "alice@example.com, bob@example.com",
		"--subject", "Lunch",
		"--body", "Noon works",
	)

	header, body := splitMIME(t, decodeCreated(t, svc))
	require.Equal(t, "alice@example.com, bob@example.com", header.Get("To"))
	require.Equal(t, "Lunch", header.Get("Subject"))
	require.Equal(t, "1.0", header.Get("MIME-Version"))
	require.Contains(t, header.Get("Content-Type"), "text/plain")
	require.Equal(t, "Noon works", body)
}

func TestCreateBodyFile(t *testing.T) {
	cfg := newTestConfig("json")
	require.NoError(t, afero.WriteFile(cfg.Fs, "note.txt", []byte("file body"), 0o644))
	svc := &fakeService{}

	runCmd(t, newCreateCmd(cfg, fakeNewSvc(svc)),
		"--to", "alice@example.com", "--subject", "Report", "--body-file", "note.txt")

	_, body := splitMIME(t, decodeCreated(t, svc))
	require.Equal(t, "file body", body)
}

func TestCreateEchoesCreatedDraft(t *testing.T) {
	svc := &fakeService{}
	out := runCmd(t, newLeafCmd(newCreateCmd, svc, "json"),
		"--to", "alice@example.com", "--body", "hi")

	row, ok := decodeJSON(t, out).(map[string]any)
	require.True(t, ok)
	keys := jsonKeys(t, row)
	require.ElementsMatch(t, []string{"id", "message_id", "snippet"}, keys)
	requireSnakeCase(t, keys)
	require.Equal(t, "draft_99", row["id"])
	require.Equal(t, "msg_99", row["message_id"])
}

func TestCreateRefusesAmbiguousBodyFlags(t *testing.T) {
	cfg := newTestConfig("json")
	require.NoError(t, afero.WriteFile(cfg.Fs, "note.txt", []byte("file body"), 0o644))
	svc := &fakeService{}

	_, err := runCmdErr(t, newCreateCmd(cfg, fakeNewSvc(svc)),
		"--to", "alice@example.com", "--body", "inline", "--body-file", "note.txt")

	require.Contains(t, err.Error(), "--body and --body-file are mutually exclusive")
	require.Nil(t, svc.created, "ambiguous input must not reach the API")
}

func TestCreateRefusesMissingBody(t *testing.T) {
	svc := &fakeService{}
	_, err := runCmdErr(t, newLeafCmd(newCreateCmd, svc, "json"), "--to", "alice@example.com")

	require.Contains(t, err.Error(), "no message body")
	require.Nil(t, svc.created)
}

func TestCreateRequiresTo(t *testing.T) {
	svc := &fakeService{}
	_, err := runCmdErr(t, newLeafCmd(newCreateCmd, svc, "json"), "--body", "hi")

	require.Contains(t, err.Error(), "no recipients")
	require.Nil(t, svc.created)
}

func TestCreateMissingBodyFile(t *testing.T) {
	svc := &fakeService{}
	_, err := runCmdErr(t, newLeafCmd(newCreateCmd, svc, "json"),
		"--to", "alice@example.com", "--body-file", "nope.txt")

	require.Contains(t, err.Error(), "reading --body-file")
	require.Nil(t, svc.created)
}

func TestCreatePropagatesAPIError(t *testing.T) {
	svc := &fakeService{err: errors.New("googleapi: Error 400")}
	_, err := runCmdErr(t, newLeafCmd(newCreateCmd, svc, "json"),
		"--to", "alice@example.com", "--body", "hi")

	require.Contains(t, err.Error(), "googleapi: Error 400")
}
