package draft

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	gmail "google.golang.org/api/gmail/v1"

	"github.com/oskarhane/google-cli/internal/providers/google/gmail/message"
	"github.com/oskarhane/google-cli/internal/providers/google/gmail/service"
	"github.com/oskarhane/google-cli/internal/subcommands/cmdtest"
)

// composeSpy records the raw MIME bytes both compose paths hand the API: the
// draft leaf stores them via CreateDraft, the send leaf via SendMessage. The
// embedded nil service.GmailService satisfies the seam; every other method is
// a stub that is never called by this test, so the nil stays untouched.
type composeSpy struct {
	service.GmailService

	created *gmail.Draft
	sent    *gmail.Message
}

func (f *composeSpy) CreateDraft(_ context.Context, draft *gmail.Draft) (*gmail.Draft, error) {
	f.created = draft
	return &gmail.Draft{Id: "draft_99", Message: draft.Message}, nil
}

func (f *composeSpy) SendMessage(_ context.Context, msg *gmail.Message) (*gmail.Message, error) {
	f.sent = msg
	return msg, nil
}

// Stubs so composeSpy satisfies both service interfaces the leaves assert;
// this test only drives create and send, so the stubs must never fire.

func (f *composeSpy) ListDrafts(context.Context, int64) ([]*gmail.Draft, error) { return nil, nil }

func (f *composeSpy) GetDraft(context.Context, string) (*gmail.Draft, error) { return nil, nil }

func (f *composeSpy) SendDraft(context.Context, *gmail.Draft) (*gmail.Message, error) {
	return nil, nil
}

func (f *composeSpy) DeleteDraft(context.Context, string) error { return nil }

func (f *composeSpy) ListMessages(context.Context, string, int64) ([]*gmail.Message, error) {
	return nil, nil
}

func (f *composeSpy) GetMessage(context.Context, string, string) (*gmail.Message, error) {
	return nil, nil
}

func (f *composeSpy) TrashMessage(context.Context, string) (*gmail.Message, error) {
	return nil, nil
}

func (f *composeSpy) UntrashMessage(context.Context, string) (*gmail.Message, error) {
	return nil, nil
}

func (f *composeSpy) DeleteMessage(context.Context, string) error { return nil }

func (f *composeSpy) ModifyMessage(context.Context, string, *gmail.ModifyMessageRequest) (*gmail.Message, error) {
	return nil, nil
}

// TestDraftAndSendShareMIMEPipeline pins the byte identity of the shared
// compose pipeline: a draft created and a message sent with the same
// to/subject/body must reach the API carrying identical Raw bytes, proving
// draft no longer composes through its own fork of message's buildMIME.
func TestDraftAndSendShareMIMEPipeline(t *testing.T) {
	cfg := cmdtest.NewTestConfig("json")
	spy := &composeSpy{}
	newDraftSvc := func(context.Context) (service.DraftService, error) { return spy, nil }
	newMessageSvc := func(context.Context) (service.MessageService, error) { return spy, nil }

	create := newCreateCmd(cfg, newDraftSvc)
	create.SetArgs([]string{
		"--to", "alice@example.com, bob@example.com",
		"--subject", "Lunch",
		"--body", "Noon works",
	})
	require.NoError(t, create.Execute())

	send := message.NewCmd(cfg, newMessageSvc)
	send.SetArgs([]string{
		"send",
		"--to", "alice@example.com, bob@example.com",
		"--subject", "Lunch",
		"--body", "Noon works",
	})
	require.NoError(t, send.Execute())

	require.NotNil(t, spy.created, "create must reach the API")
	require.NotNil(t, spy.created.Message)
	require.NotEmpty(t, spy.created.Message.Raw)
	require.NotNil(t, spy.sent, "send must reach the API")
	require.Equal(t, spy.created.Message.Raw, spy.sent.Raw,
		"draft-created and message-sent MIME must be byte-identical")
}
