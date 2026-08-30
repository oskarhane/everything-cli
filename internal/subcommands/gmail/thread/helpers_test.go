package thread

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/spf13/cobra"

	gmail "google.golang.org/api/gmail/v1"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/output"
	"github.com/oskarhane/google-cli/internal/subcommands/cmdtest"
	"github.com/oskarhane/google-cli/internal/subcommands/gmail/service"
)

// TestMain neutralizes format auto-detection so the host's harness env and
// TTY cannot flip output expectations.
func TestMain(m *testing.M) {
	output.IsAgent = func() bool { return false }
	output.StdoutIsTerminal = func() bool { return false }
	os.Exit(m.Run())
}

// fakeService is the hermetic service.ThreadService double: it serves seeded
// threads and records every call for assertions. The embedded nil
// service.GmailService satisfies the label-shaped seam the parent hands down;
// thread leaves never call those methods, so it stays nil.
type fakeService struct {
	service.GmailService

	threads    []*gmail.Thread // served by ListThreads and GetThread-by-id
	err        error           // when set, every call fails
	listQ      string          // last ListThreads query
	listLabels []string        // last ListThreads label ids
	listMax    int64           // last ListThreads max
	getID      string          // last GetThread id
}

func (f *fakeService) ListThreads(_ context.Context, q string, labelIDs []string, maxResults int64) ([]*gmail.Thread, error) {
	f.listQ, f.listLabels, f.listMax = q, labelIDs, maxResults
	if f.err != nil {
		return nil, f.err
	}
	if int64(len(f.threads)) > maxResults {
		return f.threads[:maxResults], nil
	}
	return f.threads, nil
}

func (f *fakeService) GetThread(_ context.Context, id string) (*gmail.Thread, error) {
	f.getID = id
	if f.err != nil {
		return nil, f.err
	}
	for _, t := range f.threads {
		if t.Id == id {
			return t, nil
		}
	}
	return nil, fmt.Errorf("googleapi: Error 404: thread %s not found", id)
}

// fakeNewSvc returns a service.Dialer[service.ThreadService] handing out svc, so leaves run hermeti-
// cally with no network and no real account store.
func fakeNewSvc(svc *fakeService) service.Dialer[service.ThreadService] {
	return func(context.Context) (service.ThreadService, error) { return svc, nil }
}

// newLeafCmd builds a leaf against a fake service, ready to execute.
func newLeafCmd(build func(*app.Config, service.Dialer[service.ThreadService]) *cobra.Command, svc *fakeService, format string) *cobra.Command {
	return build(cmdtest.NewTestConfig(format), fakeNewSvc(svc))
}

// seedThreads returns a small realistic thread set; thread_1 carries parsed
// payload headers for get.
func seedThreads() []*gmail.Thread {
	return []*gmail.Thread{
		{
			Id:       "thread_1",
			Snippet:  "Invoice attached",
			Messages: []*gmail.Message{seedMessage("msg_1", "boss@corp.example", "Invoice", "Mon, 24 Aug 2026 09:00:00 +0000")},
		},
		{
			Id:       "thread_2",
			Snippet:  "Lunch tomorrow?",
			Messages: []*gmail.Message{seedMessage("msg_2", "alice@example.com", "Lunch", "Tue, 25 Aug 2026 12:00:00 +0000")},
		},
	}
}

// seedMessage returns one thread message with parsed payload headers.
func seedMessage(id, from, subject, date string) *gmail.Message {
	return &gmail.Message{
		Id:      id,
		Snippet: subject + " — opening lines",
		Payload: &gmail.MessagePart{
			Headers: []*gmail.MessagePartHeader{
				{Name: "From", Value: from},
				{Name: "Subject", Value: subject},
				{Name: "Date", Value: date},
			},
		},
	}
}
