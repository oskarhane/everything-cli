package label

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/spf13/cobra"

	gmail "google.golang.org/api/gmail/v1"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/output"
	"github.com/oskarhane/google-cli/internal/providers/google/gmail/service"
	"github.com/oskarhane/google-cli/internal/subcommands/cmdtest"
)

// TestMain neutralizes format auto-detection so the host's harness env and
// TTY cannot flip output expectations.
func TestMain(m *testing.M) {
	output.IsAgent = func() bool { return false }
	output.StdoutIsTerminal = func() bool { return false }
	os.Exit(m.Run())
}

// fakeService is the hermetic service.GmailService double: it serves seeded
// labels and records writes for assertions.
type fakeService struct {
	labels       []*gmail.Label // served by ListLabels and GetLabel-by-id
	getErr       error          // when set, GetLabel always fails
	created      *gmail.Label   // last CreateLabel request
	updatedID    string         // last UpdateLabel id
	updated      *gmail.Label   // last UpdateLabel request
	deleteCalled bool
	deletedID    string
	deleteErr    error
}

func (f *fakeService) ListLabels(context.Context) ([]*gmail.Label, error) {
	return f.labels, nil
}

func (f *fakeService) GetLabel(_ context.Context, id string) (*gmail.Label, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	for _, l := range f.labels {
		if l.Id == id {
			return l, nil
		}
	}
	return nil, fmt.Errorf("googleapi: Error 404: label %s not found", id)
}

func (f *fakeService) CreateLabel(_ context.Context, label *gmail.Label) (*gmail.Label, error) {
	f.created = label
	return &gmail.Label{Id: "Label_99", Name: label.Name, Type: "user"}, nil
}

func (f *fakeService) UpdateLabel(_ context.Context, id string, label *gmail.Label) (*gmail.Label, error) {
	f.updatedID = id
	f.updated = label
	return label, nil
}

func (f *fakeService) DeleteLabel(_ context.Context, id string) error {
	f.deleteCalled = true
	f.deletedID = id
	return f.deleteErr
}

// fakeNewSvc returns a service.Dialer[service.GmailService] handing out svc, so leaves run hermeti-
// cally with no network and no real account store.
func fakeNewSvc(svc *fakeService) service.Dialer[service.GmailService] {
	return func(context.Context) (service.GmailService, error) { return svc, nil }
}

// newLeafCmd builds a leaf against a fake service, ready to execute.
func newLeafCmd(build func(*app.Config, service.Dialer[service.GmailService]) *cobra.Command, svc *fakeService, format string) *cobra.Command {
	return build(cmdtest.NewTestConfig(format), fakeNewSvc(svc))
}

// seedLabels returns a small realistic label set.
func seedLabels() []*gmail.Label {
	return []*gmail.Label{
		{Id: "INBOX", Name: "INBOX", Type: "system", MessagesTotal: 12, MessagesUnread: 3, ThreadsTotal: 9},
		{Id: "Label_7", Name: "Travel", Type: "user", MessagesTotal: 5, MessagesUnread: 0, ThreadsTotal: 4},
	}
}
