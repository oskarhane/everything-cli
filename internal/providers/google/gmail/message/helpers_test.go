package message

import (
	"context"
	"encoding/base64"
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

// fakeService is the hermetic service.MessageService double: it serves seeded
// messages and records every write for assertions. The embedded nil
// service.GmailService satisfies the label-shaped seam the parent hands down;
// message leaves never call those methods, so it stays nil.
type fakeService struct {
	service.GmailService

	messages    []*gmail.Message // served by ListMessages and GetMessage-by-id
	err         error            // when set, every call fails
	listQ       string           // last ListMessages query
	listMax     int64            // last ListMessages max
	getFormat   string           // last GetMessage format
	sent        *gmail.Message   // last SendMessage request
	trashedID   string
	untrashedID string
	deleted     bool
	deletedID   string
	modifiedID  string
	modified    *gmail.ModifyMessageRequest // last ModifyMessage request
}

func (f *fakeService) ListMessages(_ context.Context, q string, maxResults int64) ([]*gmail.Message, error) {
	f.listQ, f.listMax = q, maxResults
	if f.err != nil {
		return nil, f.err
	}
	if int64(len(f.messages)) > maxResults {
		return f.messages[:maxResults], nil
	}
	return f.messages, nil
}

func (f *fakeService) GetMessage(_ context.Context, id, format string) (*gmail.Message, error) {
	f.getFormat = format
	if f.err != nil {
		return nil, f.err
	}
	for _, m := range f.messages {
		if m.Id == id {
			return m, nil
		}
	}
	return nil, fmt.Errorf("googleapi: Error 404: message %s not found", id)
}

func (f *fakeService) SendMessage(_ context.Context, message *gmail.Message) (*gmail.Message, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.sent = message
	return &gmail.Message{Id: "msg_99", ThreadId: "thread_99", Snippet: "Sent", LabelIds: []string{"SENT"}}, nil
}

func (f *fakeService) TrashMessage(_ context.Context, id string) (*gmail.Message, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.trashedID = id
	return &gmail.Message{Id: id, LabelIds: []string{"TRASH"}}, nil
}

func (f *fakeService) UntrashMessage(_ context.Context, id string) (*gmail.Message, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.untrashedID = id
	return &gmail.Message{Id: id, LabelIds: []string{"INBOX"}}, nil
}

func (f *fakeService) DeleteMessage(_ context.Context, id string) error {
	if f.err != nil {
		return f.err
	}
	f.deleted, f.deletedID = true, id
	return nil
}

func (f *fakeService) ModifyMessage(_ context.Context, id string, req *gmail.ModifyMessageRequest) (*gmail.Message, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.modifiedID, f.modified = id, req
	return &gmail.Message{Id: id, ThreadId: "thread_1", Snippet: "Invoice attached", LabelIds: []string{"INBOX", "STARRED"}}, nil
}

// fakeNewSvc returns a service.Dialer[service.MessageService] handing out svc, so leaves run hermeti-
// cally with no network and no real account store.
func fakeNewSvc(svc *fakeService) service.Dialer[service.MessageService] {
	return func(context.Context) (service.MessageService, error) { return svc, nil }
}

// newLeafCmd builds a leaf against a fake service, ready to execute.
func newLeafCmd(build func(*app.Config, service.Dialer[service.MessageService]) *cobra.Command, svc *fakeService, format string) *cobra.Command {
	return build(cmdtest.NewTestConfig(format), fakeNewSvc(svc))
}

// seedMessages returns a small realistic message set.
func seedMessages() []*gmail.Message {
	return []*gmail.Message{
		{Id: "msg_1", ThreadId: "thread_1", Snippet: "Invoice attached", LabelIds: []string{"INBOX", "UNREAD"}},
		{Id: "msg_2", ThreadId: "thread_2", Snippet: "Lunch tomorrow?", LabelIds: []string{"INBOX"}},
	}
}

// seedDetailMessage returns one message carrying parsed payload headers and a
// base64url raw body, for get and get --raw.
func seedDetailMessage() *gmail.Message {
	raw := "From: boss@corp.example\r\nTo: me@example.com\r\nSubject: Invoice\r\nDate: Mon, 24 Aug 2026 09:00:00 +0000\r\n\r\nPlease review the invoice."
	return &gmail.Message{
		Id:       "msg_1",
		ThreadId: "thread_1",
		Snippet:  "Please review",
		LabelIds: []string{"INBOX", "UNREAD"},
		Payload: &gmail.MessagePart{
			Headers: []*gmail.MessagePartHeader{
				{Name: "From", Value: "boss@corp.example"},
				{Name: "To", Value: "me@example.com"},
				{Name: "Subject", Value: "Invoice"},
				{Name: "Date", Value: "Mon, 24 Aug 2026 09:00:00 +0000"},
			},
		},
		Raw: base64.RawURLEncoding.EncodeToString([]byte(raw)),
	}
}
