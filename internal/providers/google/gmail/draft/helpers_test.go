package draft

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"net/textproto"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

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

// fakeService is the hermetic service.DraftService double: it serves seeded
// drafts and records every write for assertions. The embedded nil
// service.GmailService satisfies the label-shaped seam the parent hands down;
// draft leaves never call those methods, so it stays nil.
type fakeService struct {
	service.GmailService

	drafts    []*gmail.Draft // served by ListDrafts and GetDraft-by-id
	err       error          // when set, every call fails
	listMax   int64          // last ListDrafts max
	getID     string         // last GetDraft id
	created   *gmail.Draft   // last CreateDraft request
	sentDraft string         // last SendDraft draft id
	deletedID string         // last DeleteDraft id
}

func (f *fakeService) ListDrafts(_ context.Context, maxResults int64) ([]*gmail.Draft, error) {
	f.listMax = maxResults
	if f.err != nil {
		return nil, f.err
	}
	if int64(len(f.drafts)) > maxResults {
		return f.drafts[:maxResults], nil
	}
	return f.drafts, nil
}

func (f *fakeService) GetDraft(_ context.Context, id string) (*gmail.Draft, error) {
	f.getID = id
	if f.err != nil {
		return nil, f.err
	}
	for _, d := range f.drafts {
		if d.Id == id {
			return d, nil
		}
	}
	return nil, fmt.Errorf("googleapi: Error 404: draft %s not found", id)
}

func (f *fakeService) CreateDraft(_ context.Context, draft *gmail.Draft) (*gmail.Draft, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.created = draft
	return &gmail.Draft{
		Id:      "draft_99",
		Message: &gmail.Message{Id: "msg_99", Snippet: "Draft stored"},
	}, nil
}

func (f *fakeService) SendDraft(_ context.Context, draft *gmail.Draft) (*gmail.Message, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.sentDraft = draft.Id
	return &gmail.Message{Id: "msg_99", ThreadId: "thread_99", Snippet: "Sent"}, nil
}

func (f *fakeService) DeleteDraft(_ context.Context, id string) error {
	if f.err != nil {
		return f.err
	}
	f.deletedID = id
	return nil
}

// fakeNewSvc returns a service.Dialer[service.DraftService] handing out svc, so leaves run hermeti-
// cally with no network and no real account store.
func fakeNewSvc(svc *fakeService) service.Dialer[service.DraftService] {
	return func(context.Context) (service.DraftService, error) { return svc, nil }
}

// newLeafCmd builds a leaf against a fake service, ready to execute.
func newLeafCmd(build func(*app.Config, service.Dialer[service.DraftService]) *cobra.Command, svc *fakeService, format string) *cobra.Command {
	return build(cmdtest.NewTestConfig(format), fakeNewSvc(svc))
}

// seedDrafts returns a small realistic draft set; draft_1's message carries
// parsed payload headers for get.
func seedDrafts() []*gmail.Draft {
	return []*gmail.Draft{
		{
			Id: "draft_1",
			Message: &gmail.Message{
				Id:      "msg_1",
				Snippet: "Invoice attached",
				Payload: &gmail.MessagePart{
					Headers: []*gmail.MessagePartHeader{
						{Name: "From", Value: "me@example.com"},
						{Name: "To", Value: "boss@corp.example"},
						{Name: "Subject", Value: "Invoice"},
						{Name: "Date", Value: "Mon, 24 Aug 2026 09:00:00 +0000"},
					},
				},
			},
		},
		{Id: "draft_2", Message: &gmail.Message{Id: "msg_2", Snippet: "Lunch tomorrow?"}},
	}
}

// decodeCreated returns the RFC 2822 message the leaf last handed the service,
// decoded from the API's base64url raw field, and asserts the wire encoding.
func decodeCreated(t *testing.T, svc *fakeService) string {
	t.Helper()
	require.NotNil(t, svc.created, "create must reach the API")
	require.NotNil(t, svc.created.Message, "create must carry the stored message")
	require.NotEmpty(t, svc.created.Message.Raw)
	decoded, err := base64.RawURLEncoding.DecodeString(svc.created.Message.Raw)
	require.NoError(t, err, "raw field must be unpadded base64url")
	return string(decoded)
}

// splitMIME splits a message into its header block and body.
func splitMIME(t *testing.T, raw string) (textproto.MIMEHeader, string) {
	t.Helper()
	headerBlock, body, ok := strings.Cut(raw, "\r\n\r\n")
	require.True(t, ok, "message must have a header/body split: %q", raw)
	// The appended blank line terminates the header block the way it appears
	// on the wire, which ReadMIMEHeader needs to stop cleanly.
	header, err := textproto.NewReader(bufio.NewReader(strings.NewReader(headerBlock + "\r\n\r\n"))).ReadMIMEHeader()
	require.NoError(t, err)
	return header, body
}
