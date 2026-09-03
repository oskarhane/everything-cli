package email

import (
	"context"
	"io"
	"time"
)

// MailService is the whole IMAP/SMTP surface this CLI uses. Leaves call
// its methods directly on the value returned by the package-var dial
// seams; per-leaf surface discipline is enforced by the test fake's
// unexpected-call guards, not by type narrowing.
type MailService interface {
	// ListMailboxes lists the account's mailbox (folder) names.
	ListMailboxes(ctx context.Context) ([]string, error)
	// ListEnvelopes lists message envelopes in one mailbox, newest first.
	// limit caps the result count (<= 0 means all).
	ListEnvelopes(ctx context.Context, mailbox string, limit int) ([]Envelope, error)
	// GetMessage fetches one full message by IMAP UID.
	GetMessage(ctx context.Context, mailbox string, uid uint32) (*Message, error)
	// SendMessage submits a composed message over SMTP.
	SendMessage(ctx context.Context, in SendInput) error
	// Close logs out of IMAP and releases the connection.
	Close() error
}

// Envelope is one entry of a mailbox listing: just the headers a list view
// needs, never a body. Field names are snake_case output keys (AGENTS.md
// casing rule).
type Envelope struct {
	UID     uint32    `json:"uid"`
	Date    time.Time `json:"date"`
	From    string    `json:"from"`
	Subject string    `json:"subject"`
	Flags   []string  `json:"flags"`
}

// Message is a fully fetched message: every decoded header field, the
// decoded plain-text body, and attachment metadata. Attachment bytes are
// never exposed here — leaves download them separately if needed.
type Message struct {
	UID         uint32              `json:"uid"`
	Headers     map[string][]string `json:"headers"`
	From        string              `json:"from"`
	To          []string            `json:"to"`
	Subject     string              `json:"subject"`
	Date        time.Time           `json:"date"`
	BodyText    string              `json:"body_text"`
	Attachments []Attachment        `json:"attachments"`
}

// Attachment is the metadata of one MIME attachment part.
type Attachment struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
}

// SendInput is one outgoing message. Body is the plain-text body; a nil
// Body sends an empty one. To/Cc entries accept plain addresses or
// "Name <addr>" forms.
type SendInput struct {
	To      []string
	Cc      []string
	Subject string
	Body    io.Reader
}
