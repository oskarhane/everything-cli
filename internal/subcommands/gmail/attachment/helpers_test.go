package attachment

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"testing"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	gmail "google.golang.org/api/gmail/v1"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/output"
	"github.com/oskarhane/google-cli/internal/subcommands/gmail/service"
)

// TestMain neutralizes format auto-detection so the host's harness env and
// TTY cannot flip output expectations.
func TestMain(m *testing.M) {
	output.IsAgent = func() bool { return false }
	output.StdoutIsTerminal = func() bool { return false }
	os.Exit(m.Run())
}

// newTestConfig returns a config with the given format and an in-memory FS,
// so --out never touches the real disk.
func newTestConfig(format string) *app.Config {
	return &app.Config{Format: format, Fs: afero.NewMemMapFs()}
}

// fakeService is the hermetic service.AttachmentService double: it serves one
// seeded attachment and records the last lookup for assertions. The embedded
// nil service.GmailService satisfies the label-shaped seam the parent hands
// down; attachment leaves never call those methods, so it stays nil.
type fakeService struct {
	service.GmailService

	content      []byte // attachment bytes served by GetAttachment
	err          error  // when set, every call fails
	messageID    string // last GetAttachment message id
	attachmentID string // last GetAttachment attachment id
}

func (f *fakeService) GetAttachment(_ context.Context, messageID, attachmentID string) (*gmail.MessagePartBody, error) {
	f.messageID, f.attachmentID = messageID, attachmentID
	if f.err != nil {
		return nil, f.err
	}
	if f.content == nil {
		return nil, fmt.Errorf("googleapi: Error 404: attachment %s not found", attachmentID)
	}
	return &gmail.MessagePartBody{
		AttachmentId: attachmentID,
		Size:         int64(len(f.content)),
		Data:         base64.RawURLEncoding.EncodeToString(f.content),
	}, nil
}

// fakeNewSvc returns a serviceFunc handing out svc, so leaves run hermeti-
// cally with no network and no real account store.
func fakeNewSvc(svc *fakeService) serviceFunc {
	return func(context.Context) (service.GmailService, error) { return svc, nil }
}

// runCmd executes a leaf cmd with its positional args and flags, returning
// everything it wrote. args must NOT include the leaf's own name: SetArgs
// feeds a single command, not a command path.
func runCmd(t *testing.T, cmd *cobra.Command, args ...string) string {
	t.Helper()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)
	require.NoError(t, cmd.Execute())
	return buf.String()
}

// runCmdErr executes cmd expecting failure, returning the error and output.
func runCmdErr(t *testing.T, cmd *cobra.Command, args ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	require.Error(t, err)
	return buf.String(), err
}
