package email

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/auth"
)

// getFixtureDate is the fixed Date of the test fixture message.
var getFixtureDate = time.Date(2026, 7, 1, 9, 15, 0, 0, time.UTC)

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

func TestMessageGet_JSON(t *testing.T) {
	_, root, out := newEmailEnv(t)
	fake := getFake(multipartGetFixture(), nil)
	stubDial(t, &dialMail, fake, nil)

	stdout, err := execute(t, root, out, "email", "message", "get", "7", "--format", "json")
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
	_, root, out := newEmailEnv(t)
	fake := getFake(multipartGetFixture(), nil)
	stubDial(t, &dialMail, fake, nil)

	_, err := execute(t, root, out, "email", "message", "get", "42", "--mailbox", "Archive", "--format", "json")
	require.NoError(t, err)
	assert.Equal(t, "Archive", fake.gotMailbox)
	assert.Equal(t, uint32(42), fake.gotUID)
}

func TestMessageGet_Table(t *testing.T) {
	_, root, out := newEmailEnv(t)
	fake := getFake(multipartGetFixture(), nil)
	stubDial(t, &dialMail, fake, nil)

	stdout, err := execute(t, root, out, "email", "message", "get", "7", "--format", "table")
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
	_, root, out := newEmailEnv(t)
	fixture := multipartGetFixture()
	fixture.BodyText = "before\x1b[31m red\x07 after\x7f"
	stubDial(t, &dialMail, getFake(fixture, nil), nil)

	// C0 control bytes must not break the TOON marshal: PrintToon deep-strips
	// them (falling back to JSON on residual error) — never a panic.
	stdout, err := execute(t, root, out, "email", "message", "get", "7", "--format", "toon")
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
			args:    []string{"email", "message", "get", "abc", "--format", "json"},
			wantErr: `invalid uid "abc"`,
		},
		{
			name:    "negative uid is a usage error",
			args:    []string{"email", "message", "get", "--format", "json", "--", "-1"},
			wantErr: `invalid uid "-1"`,
		},
		{
			name:    "dial failure propagates",
			args:    []string{"email", "message", "get", "7", "--format", "json"},
			dialErr: errors.New("dial refused"),
			wantErr: "dial refused",
		},
		{
			name:    "service failure propagates",
			args:    []string{"email", "message", "get", "7", "--format", "json"},
			svcErr:  errors.New("no such message"),
			wantErr: "no such message",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, root, out := newEmailEnv(t)
			fake := getFake(multipartGetFixture(), tt.svcErr)
			stubDial(t, &dialMail, fake, tt.dialErr)

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
	_, root, out := newEmailEnv(t)
	stubDial(t, &dialMail, getFake(multipartGetFixture(), nil), nil)

	_, err := execute(t, root, out, "email", "message", "get", "--format", "json")
	require.Error(t, err)

	_, err = execute(t, root, out, "email", "message", "get", "1", "2", "--format", "json")
	require.Error(t, err)
}

// TestMessageGet_TableRedactsSecret: the table-mode body print bypasses
// the output.Print chokepoint, so it must redact on its own — a reset
// email quoting the account password must render `***`, never the value.
// (JSON/TOON already pass through the central redactor; this pins the
// table path.)
func TestMessageGet_TableRedactsSecret(t *testing.T) {
	_, root, out := newEmailEnv(t)
	const secret = "table-redact-distinctive-secret-7e1"
	auth.RegisterSecret(secret)
	fixture := multipartGetFixture()
	fixture.BodyText = "Your new password is " + secret + " — keep it safe."
	stubDial(t, &dialMail, getFake(fixture, nil), nil)

	stdout, err := execute(t, root, out, "email", "message", "get", "7", "--format", "table")
	require.NoError(t, err)
	assert.NotContains(t, stdout, secret, "the secret must never reach table output")
	assert.Contains(t, stdout, "***")
	assert.Contains(t, stdout, "Your new password is")

	// JSON redaction (via the central output chokepoint) is unchanged.
	stdout, err = execute(t, root, out, "email", "message", "get", "7", "--format", "json")
	require.NoError(t, err)
	assert.NotContains(t, stdout, secret)
	assert.Contains(t, stdout, "***")
}

// TestMessageGet_ToonControlBytesInSubject proves stripping applies to
// header-shaped strings too, not just the body.
func TestMessageGet_ToonControlBytesInSubject(t *testing.T) {
	_, root, out := newEmailEnv(t)
	fixture := multipartGetFixture()
	fixture.Subject = "sneaky\x1b[0m subject"
	stubDial(t, &dialMail, getFake(fixture, nil), nil)

	stdout, err := execute(t, root, out, "email", "message", "get", "7", "--format", "toon")
	require.NoError(t, err)
	assert.NotContains(t, stdout, "\x1b")
	assert.True(t, strings.Contains(stdout, "sneaky"))
}
