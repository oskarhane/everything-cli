package email

import (
	"context"
	"fmt"
	"io"
	"time"
)

// MailService is the whole IMAP/SMTP surface this CLI uses. Leaves never
// consume it directly: they narrow it to one of the per-concern interfaces
// below via As, so fakes model a single concern instead of the union.
type MailService interface {
	MailboxLister
	EnvelopeLister
	MessageGetter
	MessageSender
	// Close logs out of IMAP and releases the connection.
	Close() error
}

// MailboxLister lists the account's mailbox (folder) names.
type MailboxLister interface {
	ListMailboxes(ctx context.Context) ([]string, error)
}

// EnvelopeLister lists message envelopes in one mailbox, newest first.
// limit caps the result count (<= 0 means all).
type EnvelopeLister interface {
	ListEnvelopes(ctx context.Context, mailbox string, limit int) ([]Envelope, error)
}

// MessageGetter fetches one full message by IMAP UID.
type MessageGetter interface {
	GetMessage(ctx context.Context, mailbox string, uid uint32) (*Message, error)
}

// MessageSender submits a composed message over SMTP.
type MessageSender interface {
	SendMessage(ctx context.Context, in SendInput) error
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

// As narrows the MailService a leaf obtained from the package-var dialMail
// seam into any of the per-concern interfaces (MailboxLister,
// EnvelopeLister, ...). The real service implements every interface; the
// assertion hands a subtree its own surface without growing the others.
// Unlike the gmail seam, the dialer is not injected into constructors:
// leaves close over dialMail directly and narrow after the fact because
// the leaf owns the persistent IMAP connection's Close.
func As[T any](svc MailService) (T, error) {
	narrowed, ok := svc.(T)
	if !ok {
		return narrowed, fmt.Errorf("mail service does not implement the requested operations")
	}
	return narrowed, nil
}
