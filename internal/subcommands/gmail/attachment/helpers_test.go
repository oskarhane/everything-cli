package attachment

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"testing"

	gmail "google.golang.org/api/gmail/v1"

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

// fakeNewSvc returns a service.Dialer[service.AttachmentService] handing out svc, so leaves run hermeti-
// cally with no network and no real account store.
func fakeNewSvc(svc *fakeService) service.Dialer[service.AttachmentService] {
	return func(context.Context) (service.AttachmentService, error) { return svc, nil }
}
