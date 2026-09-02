package email

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"slices"
	"strconv"

	imap "github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	_ "github.com/emersion/go-message/charset" // decode common charsets to UTF-8 when parsing messages
	"github.com/emersion/go-message/mail"
	"github.com/emersion/go-sasl"
	smtp "github.com/emersion/go-smtp"
)

// adapter.go is the sole importer of the emersion mail stack (go-imap/v2,
// go-smtp, go-message, go-sasl): upstream v2-beta API churn touches exactly
// this file, and every other email file talks to the MailService seam.
//
// TLS is mandatory: IMAP always uses implicit TLS (DialTLS) and SMTP uses
// implicit TLS on the submissions port, STARTTLS otherwise. There is no
// plaintext path by design — a refused connection beats a leaked password.

// smtpImplicitTLSPort switches SMTP from STARTTLS (submission, 587) to
// implicit TLS (submissions, 465).
const smtpImplicitTLSPort = 465

// smtpUsesImplicitTLS maps a configured port to its transport. It is a
// package seam so tests can pin the mapping onto a loopback port; the
// production mapping is exactly "465 is implicit TLS, everything else is
// STARTTLS".
var smtpUsesImplicitTLS = func(port int) bool {
	return port == smtpImplicitTLSPort
}

// tlsConfigFor builds the client TLS config for a server host. It is a
// package seam so tests can pin a loopback CA; production always verifies
// against the system roots.
var tlsConfigFor = func(host string) *tls.Config {
	return &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}
}

// mailService is the concrete MailService over the emersion IMAP/SMTP
// libraries. The IMAP connection is dialed and logged in once at
// construction and shared by the read operations; SMTP is dialed per send
// because submission sessions are one-shot.
type mailService struct {
	imapClient *imapclient.Client
	creds      *credentials
}

// Compile-time proof that mailService satisfies the seam.
var _ MailService = (*mailService)(nil)

// newMailService dials the account's IMAP server over implicit TLS and
// logs in. The password is already registered for redaction by
// loadCredentials; it never appears in an error string here.
func newMailService(creds *credentials) (*mailService, error) {
	client, err := dialIMAPTLS(creds.IMAP)
	if err != nil {
		return nil, err
	}
	if err := client.Login(creds.Username, creds.Password).Wait(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("imap login as %q: %w", creds.Username, err)
	}
	return &mailService{imapClient: client, creds: creds}, nil
}

// dialIMAPTLS connects with implicit TLS — the only IMAP transport this
// provider supports.
func dialIMAPTLS(srv serverConfig) (*imapclient.Client, error) {
	port := srv.Port
	if port == 0 {
		port = defaultIMAPPort
	}
	addr := net.JoinHostPort(srv.Host, strconv.Itoa(port))
	client, err := imapclient.DialTLS(addr, &imapclient.Options{TLSConfig: tlsConfigFor(srv.Host)})
	if err != nil {
		return nil, fmt.Errorf("connecting to imap server %s: %w", addr, err)
	}
	return client, nil
}

func (s *mailService) Close() error {
	if err := s.imapClient.Logout().Wait(); err != nil {
		_ = s.imapClient.Close()
		return fmt.Errorf("imap logout: %w", err)
	}
	return s.imapClient.Close()
}

func (s *mailService) ListMailboxes(_ context.Context) ([]string, error) {
	data, err := s.imapClient.List("", "*", nil).Collect()
	if err != nil {
		return nil, fmt.Errorf("listing mailboxes: %w", err)
	}
	names := make([]string, 0, len(data))
	for _, d := range data {
		// \Noselect entries are hierarchy placeholders, not openable
		// mailboxes.
		if hasMailboxAttr(d.Attrs, imap.MailboxAttrNoSelect) {
			continue
		}
		names = append(names, d.Mailbox)
	}
	slices.Sort(names)
	return names, nil
}

// ListEnvelopes returns the newest envelopes first. limit caps how many
// are fetched (<= 0 means all): the fetch asks the server for just the
// tail of the mailbox instead of walking every message.
func (s *mailService) ListEnvelopes(_ context.Context, mailbox string, limit int) ([]Envelope, error) {
	selected, err := s.imapClient.Select(mailbox, &imap.SelectOptions{ReadOnly: true}).Wait()
	if err != nil {
		return nil, fmt.Errorf("selecting mailbox %q: %w", mailbox, err)
	}
	if selected.NumMessages == 0 {
		return []Envelope{}, nil
	}
	start := uint32(1)
	if limit > 0 && selected.NumMessages > uint32(limit) {
		start = selected.NumMessages - uint32(limit) + 1
	}
	var seq imap.SeqSet
	seq.AddRange(start, 0) // 0 encodes "*": start through the newest message
	fetchOptions := &imap.FetchOptions{UID: true, Envelope: true, Flags: true}
	msgs, err := s.imapClient.Fetch(seq, fetchOptions).Collect()
	if err != nil {
		return nil, fmt.Errorf("fetching envelopes in %q: %w", mailbox, err)
	}
	envelopes := make([]Envelope, 0, len(msgs))
	for _, m := range msgs {
		env := Envelope{
			UID:   uint32(m.UID),
			Flags: flagStrings(m.Flags),
		}
		if m.Envelope != nil {
			env.Date = m.Envelope.Date
			env.Subject = m.Envelope.Subject
			env.From = formatIMAPAddress(m.Envelope.From)
		}
		envelopes = append(envelopes, env)
	}
	slices.Reverse(envelopes) // UIDs arrive ascending; the newest leads.
	return envelopes, nil
}

func (s *mailService) GetMessage(_ context.Context, mailbox string, uid uint32) (*Message, error) {
	if _, err := s.imapClient.Select(mailbox, &imap.SelectOptions{ReadOnly: true}).Wait(); err != nil {
		return nil, fmt.Errorf("selecting mailbox %q: %w", mailbox, err)
	}
	section := &imap.FetchItemBodySection{} // empty = BODY[], the full RFC 5322 message
	fetchOptions := &imap.FetchOptions{
		UID:         true,
		BodySection: []*imap.FetchItemBodySection{section},
	}
	msgs, err := s.imapClient.Fetch(imap.UIDSetNum(imap.UID(uid)), fetchOptions).Collect()
	if err != nil {
		return nil, fmt.Errorf("fetching message uid %d in %q: %w", uid, mailbox, err)
	}
	if len(msgs) == 0 {
		return nil, fmt.Errorf("no message with uid %d in %q", uid, mailbox)
	}
	raw := msgs[0].FindBodySection(section)
	if raw == nil {
		return nil, fmt.Errorf("message uid %d in %q returned no body", uid, mailbox)
	}
	return parseMessage(uint32(msgs[0].UID), raw)
}

func (s *mailService) SendMessage(_ context.Context, in SendInput) error {
	raw, rcpts, err := composeMessage(s.creds.Username, in)
	if err != nil {
		return err
	}
	client, err := dialSMTPTLS(s.creds.SMTP)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()
	if err := client.Auth(sasl.NewPlainClient("", s.creds.Username, s.creds.Password)); err != nil {
		return fmt.Errorf("smtp auth as %q: %w", s.creds.Username, err)
	}
	if err := client.SendMail(s.creds.Username, rcpts, bytes.NewReader(raw)); err != nil {
		return fmt.Errorf("submitting message: %w", err)
	}
	return client.Quit()
}

// dialSMTPTLS connects over TLS: implicit TLS on the submissions port,
// STARTTLS everywhere else. There is no plaintext path.
func dialSMTPTLS(srv serverConfig) (*smtp.Client, error) {
	port := srv.Port
	if port == 0 {
		port = defaultSMTPPort
	}
	addr := net.JoinHostPort(srv.Host, strconv.Itoa(port))
	tlsCfg := tlsConfigFor(srv.Host)
	var (
		client *smtp.Client
		err    error
	)
	if smtpUsesImplicitTLS(port) {
		client, err = smtp.DialTLS(addr, tlsCfg)
	} else {
		client, err = smtp.DialStartTLS(addr, tlsCfg)
	}
	if err != nil {
		return nil, fmt.Errorf("connecting to smtp server %s: %w", addr, err)
	}
	return client, nil
}

// parseMessage decodes a raw RFC 5322 message into the Message output
// shape: the first text/plain part becomes body_text, every attachment
// part contributes its metadata (size counted on the decoded bytes).
func parseMessage(uid uint32, raw []byte) (*Message, error) {
	mr, err := mail.CreateReader(bytes.NewReader(raw))
	if mr == nil {
		return nil, fmt.Errorf("parsing message: %w", err)
	}
	msg := &Message{UID: uid}
	if subject, err := mr.Header.Subject(); err == nil {
		msg.Subject = subject
	}
	if date, err := mr.Header.Date(); err == nil {
		msg.Date = date
	}
	msg.From = firstAddress(mr.Header, "From")
	msg.To = addressStrings(mr.Header, "To")

	for {
		part, perr := mr.NextPart()
		if errors.Is(perr, io.EOF) {
			break
		}
		if part == nil {
			return nil, fmt.Errorf("reading message part: %w", perr)
		}
		switch h := part.Header.(type) {
		case *mail.InlineHeader:
			contentType, _, _ := h.ContentType()
			// Only the first text/plain part is the body; later inline
			// parts (e.g. text/html alternatives) are ignored.
			if msg.BodyText == "" && (contentType == "" || contentType == "text/plain") {
				body, err := io.ReadAll(part.Body)
				if err != nil {
					return nil, fmt.Errorf("reading body part: %w", err)
				}
				msg.BodyText = string(body)
			}
		case *mail.AttachmentHeader:
			filename, _ := h.Filename()
			contentType, _, _ := h.ContentType()
			body, err := io.ReadAll(part.Body)
			if err != nil {
				return nil, fmt.Errorf("reading attachment %q: %w", filename, err)
			}
			msg.Attachments = append(msg.Attachments, Attachment{
				Filename:    filename,
				ContentType: contentType,
				Size:        int64(len(body)),
			})
		}
	}
	return msg, nil
}

// composeMessage renders in as an RFC 5322 message and returns the raw
// bytes plus the plain RCPT addresses (header display names are stripped).
func composeMessage(from string, in SendInput) ([]byte, []string, error) {
	if len(in.To) == 0 {
		return nil, nil, errors.New("at least one --to recipient is required")
	}
	var buf bytes.Buffer
	var h mail.Header
	h.SetAddressList("From", []*mail.Address{{Address: from}})
	if in.Subject != "" {
		h.SetSubject(in.Subject)
	}
	to, rcpts, err := parseAddresses(in.To)
	if err != nil {
		return nil, nil, err
	}
	h.SetAddressList("To", to)
	if len(in.Cc) > 0 {
		cc, ccRcpts, err := parseAddresses(in.Cc)
		if err != nil {
			return nil, nil, err
		}
		h.SetAddressList("Cc", cc)
		rcpts = append(rcpts, ccRcpts...)
	}
	h.Set("Content-Type", "text/plain; charset=utf-8")
	w, err := mail.CreateSingleInlineWriter(&buf, h)
	if err != nil {
		return nil, nil, fmt.Errorf("composing message: %w", err)
	}
	if in.Body != nil {
		if _, err := io.Copy(w, in.Body); err != nil {
			return nil, nil, fmt.Errorf("composing message body: %w", err)
		}
	}
	if err := w.Close(); err != nil {
		return nil, nil, fmt.Errorf("composing message: %w", err)
	}
	return buf.Bytes(), rcpts, nil
}

// parseAddresses parses RFC 5322 address lists into header addresses and
// plain RCPT addresses.
func parseAddresses(raw []string) ([]*mail.Address, []string, error) {
	var (
		addrs []*mail.Address
		rcpts []string
	)
	for _, entry := range raw {
		parsed, err := mail.ParseAddressList(entry)
		if err != nil {
			return nil, nil, fmt.Errorf("parsing address %q: %w", entry, err)
		}
		for _, a := range parsed {
			addrs = append(addrs, a)
			rcpts = append(rcpts, a.Address)
		}
	}
	return addrs, rcpts, nil
}

func hasMailboxAttr(attrs []imap.MailboxAttr, want imap.MailboxAttr) bool {
	return slices.Contains(attrs, want)
}

func flagStrings(flags []imap.Flag) []string {
	out := make([]string, 0, len(flags))
	for _, f := range flags {
		out = append(out, string(f))
	}
	return out
}

// formatIMAPAddress renders the first envelope address as
// "Name <mailbox@host>" (or bare "mailbox@host" when there is no display
// name), matching how the mail package renders addresses.
func formatIMAPAddress(addrs []imap.Address) string {
	for _, a := range addrs {
		addr := a.Addr()
		if addr == "" {
			continue
		}
		if a.Name != "" {
			return fmt.Sprintf("%s <%s>", a.Name, addr)
		}
		return addr
	}
	return ""
}

func firstAddress(h mail.Header, key string) string {
	addrs, err := h.AddressList(key)
	if err != nil || len(addrs) == 0 {
		return ""
	}
	return formatMailAddress(addrs[0])
}

func addressStrings(h mail.Header, key string) []string {
	addrs, err := h.AddressList(key)
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(addrs))
	for _, a := range addrs {
		out = append(out, formatMailAddress(a))
	}
	return out
}

// formatMailAddress renders "Name <addr>" without net/mail's quoting, or
// the bare address when there is no display name.
func formatMailAddress(a *mail.Address) string {
	if a.Name != "" {
		return fmt.Sprintf("%s <%s>", a.Name, a.Address)
	}
	return a.Address
}
