package attachment

import (
	"context"
	"os"
	"testing"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

// TestMain neutralizes format auto-detection so the host's harness env and
// TTY cannot flip output expectations.
func TestMain(m *testing.M) {
	output.IsAgent = func() bool { return false }
	output.StdoutIsTerminal = func() bool { return false }
	os.Exit(m.Run())
}

// fakeService is the hermetic service.AttachmentService double: it records
// the issue id and input it receives and echoes the created attachment.
type fakeService struct {
	err     error // when set, every call fails
	gotID   string
	created service.CreateAttachmentInput
}

func (f *fakeService) CreateAttachment(_ context.Context, issueID string, in service.CreateAttachmentInput) (*service.Attachment, error) {
	f.gotID = issueID
	f.created = in
	if f.err != nil {
		return nil, f.err
	}
	created := seedAttachment()
	created.Title = in.Title
	created.URL = in.URL
	created.Subtitle = in.Subtitle
	return &created, nil
}

// fakeNewSvc hands out svc so leaves run hermetically with no network and no
// real account store.
func fakeNewSvc(svc *fakeService) service.Dialer[service.AttachmentService] {
	return func(context.Context) (service.AttachmentService, error) { return svc, nil }
}

// newLeafCmd builds a leaf against a fake service, ready to execute.
func newLeafCmd(build func(*app.Config, service.Dialer[service.AttachmentService]) *cobra.Command, svc *fakeService, format string) *cobra.Command {
	return build(cmdtest.NewTestConfig(format), fakeNewSvc(svc))
}

// seedAttachment returns one attachment carrying every rendered field.
func seedAttachment() service.Attachment {
	return service.Attachment{
		ID:        "attachment_1",
		Title:     "RFC",
		URL:       "https://example.com/rfc",
		Subtitle:  "Design doc",
		CreatedAt: "2026-08-01T10:00:00.000Z",
		UpdatedAt: "2026-08-02T10:00:00.000Z",
	}
}
