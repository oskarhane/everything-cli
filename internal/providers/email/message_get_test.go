package email

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/app"
)

// getFixtureDate is the fixed Date of the test fixture message.
var getFixtureDate = time.Date(2026, 7, 1, 9, 15, 0, 0, time.UTC)

// fakeGetService is a MailService fake for message-get tests: GetMessage is
// the only live method; the rest exist to satisfy the union interface the
// dialMail seam returns. gotMailbox/gotUID record the call so tests prove
// flag plumbing; closed proves the leaf's deferred Close ran.
type fakeGetService struct {
	msg        *Message
	err        error
	gotMailbox string
	gotUID     uint32
	closed     bool
}

func (f *fakeGetService) ListMailboxes(context.Context) ([]string, error) {
	return nil, errors.New("fakeGetService: ListMailboxes not implemented")
}

func (f *fakeGetService) ListEnvelopes(context.Context, string, int) ([]Envelope, error) {
	return nil, errors.New("fakeGetService: ListEnvelopes not implemented")
}

func (f *fakeGetService) GetMessage(_ context.Context, mailbox string, uid uint32) (*Message, error) {
	f.gotMailbox, f.gotUID = mailbox, uid
	return f.msg, f.err
}

func (f *fakeGetService) SendMessage(context.Context, SendInput) error {
	return errors.New("fakeGetService: SendMessage not implemented")
}

func (f *fakeGetService) Close() error {
	f.closed = true
	return nil
}

// stubGetDial swaps the dialMail seam for the test's lifetime so leaves run
// against the fake service instead of a real IMAP dial.
func stubGetDial(t *testing.T, svc MailService, err error) {
	t.Helper()
	saved := dialMail
	dialMail = func(context.Context, *app.Config) (MailService, error) { return svc, err }
	t.Cleanup(func() { dialMail = saved })
}

// multipartGetFixture models a decoded multipart/mixed message: BodyText is
// the decoded text part and the attachment carries metadata only.
func multipartGetFixture() *Message {
	return &Message{
		UID:      7,
		From:     "Carol <carol@example.com>",
		To:       []string{"bob@example.com", "dave@example.com"},
		Subject:  "Report attached",
		Date:     getFixtureDate,
		BodyText: "See the attached report.",
		Attachments: []Attachment{
			{Filename: "report.pdf", ContentType: "application/pdf", Size: 14},
		},
	}
}

// mountGetLeaf mounts the leaf on a minimal message parent under the env's
// root: the real message parent (message.go) is wired by a later change, so
// tests build their own rather than depend on the sibling-owned file. The
// root comes from newEmailEnv so stdout capture stays in place.
func mountGetLeaf(cfg *app.Config, root *cobra.Command) {
	parent := &cobra.Command{Use: "message"}
	parent.AddCommand(newMessageGetCmd(cfg))
	root.AddCommand(parent)
}

func TestMessageGet_JSON(t *testing.T) {
	cfg, root, out := newEmailEnv(t)
	fake := &fakeGetService{msg: multipartGetFixture()}
	stubGetDial(t, fake, nil)
	mountGetLeaf(cfg, root)

	stdout, err := execute(t, root, out, "message", "get", "7", "--format", "json")
	require.NoError(t, err)

	// Flag and positional plumbing reached the service.
	assert.Equal(t, "INBOX", fake.gotMailbox)
	assert.Equal(t, uint32(7), fake.gotUID)
	assert.True(t, fake.closed, "leaf must close the service")

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &got))
	assert.Equal(t, float64(7), got["uid"])
	assert.Equal(t, "Carol <carol@example.com>", got["from"])
	assert.Equal(t, []any{"bob@example.com", "dave@example.com"}, got["to"])
	assert.Equal(t, "Report attached", got["subject"])
	assert.Equal(t, getFixtureDate.Format(time.RFC3339), got["date"])
	assert.Equal(t, "See the attached report.", got["body_text"])
	atts, ok := got["attachments"].([]any)
	require.True(t, ok, "attachments = %T", got["attachments"])
	require.Len(t, atts, 1)
	att := atts[0].(map[string]any)
	assert.Equal(t, "report.pdf", att["filename"])
	assert.Equal(t, "application/pdf", att["content_type"])
	assert.Equal(t, float64(14), att["size"])
	// The raw headers map is not part of the rendered shape.
	assert.NotContains(t, got, "headers")
}

func TestMessageGet_MailboxFlag(t *testing.T) {
	cfg, root, out := newEmailEnv(t)
	fake := &fakeGetService{msg: multipartGetFixture()}
	stubGetDial(t, fake, nil)
	mountGetLeaf(cfg, root)

	_, err := execute(t, root, out, "message", "get", "42", "--mailbox", "Archive", "--format", "json")
	require.NoError(t, err)
	assert.Equal(t, "Archive", fake.gotMailbox)
	assert.Equal(t, uint32(42), fake.gotUID)
}

func TestMessageGet_Table(t *testing.T) {
	cfg, root, out := newEmailEnv(t)
	fake := &fakeGetService{msg: multipartGetFixture()}
	stubGetDial(t, fake, nil)
	mountGetLeaf(cfg, root)

	stdout, err := execute(t, root, out, "message", "get", "7", "--format", "table")
	require.NoError(t, err)
	// go-pretty StyleLight upper-cases header cells.
	for _, header := range []string{"UID", "FROM", "TO", "SUBJECT", "DATE"} {
		assert.Contains(t, stdout, header)
	}
	assert.Contains(t, stdout, "Carol <carol@example.com>")
	// The decoded text body renders below the header table.
	assert.Contains(t, stdout, "See the attached report.")
}

func TestMessageGet_Toon_ControlBytesInBody(t *testing.T) {
	cfg, root, out := newEmailEnv(t)
	fixture := multipartGetFixture()
	fixture.BodyText = "before\x1b[31m red\x07 after\x7f"
	fake := &fakeGetService{msg: fixture}
	stubGetDial(t, fake, nil)
	mountGetLeaf(cfg, root)

	// C0 control bytes must not break the TOON marshal: PrintToon deep-strips
	// them (falling back to JSON on residual error) — never a panic.
	stdout, err := execute(t, root, out, "message", "get", "7", "--format", "toon")
	require.NoError(t, err)
	assert.NotContains(t, stdout, "\x1b")
	assert.NotContains(t, stdout, "\x07")
	assert.NotContains(t, stdout, "\x7f")
	assert.Contains(t, stdout, "body_text")
	assert.Contains(t, stdout, "before")
}

func TestMessageGet_Errors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		dialErr error
		svcErr  error
		wantErr string
	}{
		{
			name:    "non-numeric uid is a usage error",
			args:    []string{"message", "get", "abc", "--format", "json"},
			wantErr: `invalid uid "abc"`,
		},
		{
			name:    "negative uid is a usage error",
			args:    []string{"message", "get", "--format", "json", "--", "-1"},
			wantErr: `invalid uid "-1"`,
		},
		{
			name:    "dial failure propagates",
			args:    []string{"message", "get", "7", "--format", "json"},
			dialErr: errors.New("dial refused"),
			wantErr: "dial refused",
		},
		{
			name:    "service failure propagates",
			args:    []string{"message", "get", "7", "--format", "json"},
			svcErr:  errors.New("no such message"),
			wantErr: "no such message",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, root, out := newEmailEnv(t)
			fake := &fakeGetService{msg: multipartGetFixture(), err: tt.svcErr}
			stubGetDial(t, fake, tt.dialErr)
			mountGetLeaf(cfg, root)

			_, err := execute(t, root, out, tt.args...)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			if tt.svcErr != nil {
				assert.True(t, fake.closed, "service must close even on GetMessage error")
			}
		})
	}
}

func TestMessageGet_ArgCount(t *testing.T) {
	cfg, root, out := newEmailEnv(t)
	stubGetDial(t, &fakeGetService{msg: multipartGetFixture()}, nil)
	mountGetLeaf(cfg, root)

	_, err := execute(t, root, out, "message", "get", "--format", "json")
	require.Error(t, err)

	_, err = execute(t, root, out, "message", "get", "1", "2", "--format", "json")
	require.Error(t, err)
}

// TestMessageGet_ToonControlBytesInSubject proves stripping applies to
// header-shaped strings too, not just the body.
func TestMessageGet_ToonControlBytesInSubject(t *testing.T) {
	cfg, root, out := newEmailEnv(t)
	fixture := multipartGetFixture()
	fixture.Subject = "sneaky\x1b[0m subject"
	fake := &fakeGetService{msg: fixture}
	stubGetDial(t, fake, nil)
	mountGetLeaf(cfg, root)

	stdout, err := execute(t, root, out, "message", "get", "7", "--format", "toon")
	require.NoError(t, err)
	assert.NotContains(t, stdout, "\x1b")
	assert.True(t, strings.Contains(stdout, "sneaky"))
}
