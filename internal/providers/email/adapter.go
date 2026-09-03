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
	"strings"
	"sync"
	"time"

	imap "github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	gomessage "github.com/emersion/go-message"
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
// construction and shared by the read operations; the send-only variant
// skips IMAP entirely because SMTP submission sessions are one-shot and
// dialed per send.
type mailService struct {
	imapClient *imapclient.Client // nil on the SMTP-only (send) path
	creds      *credentials
}

// Compile-time proof that mailService satisfies the seam.
var _ MailService = (*mailService)(nil)

// newMailService dials the account's IMAP server over implicit TLS and
// logs in. The password is already registered for redaction by
// loadCredentials; it never appears in an error string here.
func newMailService(ctx context.Context, creds *credentials) (*mailService, error) {
	client, err := dialIMAPTLS(ctx, creds.IMAP)
	if err != nil {
		return nil, err
	}
	if err := client.Login(creds.Username, creds.Password).Wait(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("imap login as %q: %w", creds.Username, err)
	}
	return &mailService{imapClient: client, creds: creds}, nil
}

// newSendMailService builds the send-only variant: no IMAP dial, no IMAP
// login. `email message send` must not depend on the IMAP server being
// reachable, and authenticating against it would be superfluous.
func newSendMailService(creds *credentials) *mailService {
	return &mailService{creds: creds}
}

// dialIMAPTLS connects with implicit TLS — the only IMAP transport this
// provider supports. The emersion imapclient API (v2.0.0-beta.8) takes no
// context on its dial helpers or commands, so the connection is dialed
// with DialContext (cancellation and deadlines reach the connect and TLS
// handshake) and handed to imapclient.New. In-flight commands are covered
// by the cancellation watchers in each method (watchCancel).
func dialIMAPTLS(ctx context.Context, srv serverConfig) (*imapclient.Client, error) {
	host, port, err := resolveDialServer(srv, defaultIMAPPort)
	if err != nil {
		return nil, fmt.Errorf("stored imap server: %w", err)
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	dialer := &tls.Dialer{
		NetDialer: &net.Dialer{Timeout: 30 * time.Second},
		Config:    tlsConfigFor(host),
	}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("connecting to imap server %s: %w", addr, err)
	}
	return imapclient.New(conn, &imapclient.Options{TLSConfig: tlsConfigFor(host)}), nil
}

// watchCancel closes the client when ctx is cancelled: the emersion
// command APIs take no context, so closing the connection is the only
// genuine way to abort in-flight network I/O on cancellation. The
// returned stop func unregisters the watcher.
func watchCancel(ctx context.Context, closeFn func() error) context.CancelFunc {
	if ctx.Done() == nil {
		return func() {}
	}
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = closeFn()
		case <-done:
		}
	}()
	var once sync.Once
	return func() { once.Do(func() { close(done) }) }
}

func (s *mailService) Close() error {
	if s.imapClient == nil {
		return nil // SMTP-only (send) service: nothing to log out of
	}
	if err := s.imapClient.Logout().Wait(); err != nil {
		_ = s.imapClient.Close()
		return fmt.Errorf("imap logout: %w", err)
	}
	return s.imapClient.Close()
}

func (s *mailService) ListMailboxes(ctx context.Context) ([]string, error) {
	stop := watchCancel(ctx, s.imapClient.Close)
	defer stop()
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
func (s *mailService) ListEnvelopes(ctx context.Context, mailbox string, limit int) ([]Envelope, error) {
	stop := watchCancel(ctx, s.imapClient.Close)
	defer stop()
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

// GetMessage fetches one message without ever transferring or buffering
// its attachments. It first fetches BODYSTRUCTURE (small metadata: part
// layout, sizes, dispositions) to locate the first inline text/plain part
// and to collect attachment metadata, then fetches only BODY[HEADER] and
// that body part. A hostile or huge message therefore cannot OOM the
// process through attachment bytes — the BODY[] full-message fetch this
// replaced buffered everything.
func (s *mailService) GetMessage(ctx context.Context, mailbox string, uid uint32) (*Message, error) {
	stop := watchCancel(ctx, s.imapClient.Close)
	defer stop()
	if _, err := s.imapClient.Select(mailbox, &imap.SelectOptions{ReadOnly: true}).Wait(); err != nil {
		return nil, fmt.Errorf("selecting mailbox %q: %w", mailbox, err)
	}
	uidSet := imap.UIDSetNum(imap.UID(uid))
	structMsgs, err := s.imapClient.Fetch(uidSet, &imap.FetchOptions{
		UID:           true,
		BodyStructure: &imap.FetchItemBodyStructure{Extended: true},
	}).Collect()
	if err != nil {
		return nil, fmt.Errorf("fetching message uid %d in %q: %w", uid, mailbox, err)
	}
	if len(structMsgs) == 0 || structMsgs[0].BodyStructure == nil {
		return nil, fmt.Errorf("no message with uid %d in %q", uid, mailbox)
	}
	bodyPath, attachments := analyzeBodyStructure(structMsgs[0].BodyStructure)

	headerSection := &imap.FetchItemBodySection{Specifier: imap.PartSpecifierHeader}
	sections := []*imap.FetchItemBodySection{headerSection}
	var mimeSection, bodySection *imap.FetchItemBodySection
	if bodyPath != nil {
		// The part's own MIME header (Content-Type/charset and
		// Content-Transfer-Encoding) travels separately from its body, so
		// both are fetched and glued back together for decoding.
		mimeSection = &imap.FetchItemBodySection{Specifier: imap.PartSpecifierMIME, Part: bodyPath}
		bodySection = &imap.FetchItemBodySection{Part: bodyPath}
		sections = append(sections, mimeSection, bodySection)
	}
	msgs, err := s.imapClient.Fetch(uidSet, &imap.FetchOptions{UID: true, BodySection: sections}).Collect()
	if err != nil {
		return nil, fmt.Errorf("fetching message uid %d in %q: %w", uid, mailbox, err)
	}
	if len(msgs) == 0 {
		return nil, fmt.Errorf("no message with uid %d in %q", uid, mailbox)
	}
	headerRaw := msgs[0].FindBodySection(headerSection)
	if headerRaw == nil {
		return nil, fmt.Errorf("message uid %d in %q returned no header", uid, mailbox)
	}
	msg, err := parseHeaderBlock(uint32(msgs[0].UID), headerRaw)
	if err != nil {
		return nil, err
	}
	msg.Attachments = attachments
	if bodyPath != nil {
		mimeRaw := msgs[0].FindBodySection(mimeSection)
		bodyRaw := msgs[0].FindBodySection(bodySection)
		if mimeRaw == nil || bodyRaw == nil {
			return nil, fmt.Errorf("message uid %d in %q returned no body part", uid, mailbox)
		}
		body, err := parseBodyPart(mimeRaw, bodyRaw)
		if err != nil {
			return nil, err
		}
		msg.BodyText = body
	}
	return msg, nil
}

func (s *mailService) SendMessage(ctx context.Context, in SendInput) error {
	raw, rcpts, err := composeMessage(s.creds.Username, in)
	if err != nil {
		return err
	}
	client, err := dialSMTPTLS(ctx, s.creds.SMTP)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()
	stop := watchCancel(ctx, client.Close)
	defer stop()
	if err := client.Auth(sasl.NewPlainClient("", s.creds.Username, s.creds.Password)); err != nil {
		return fmt.Errorf("smtp auth as %q: %w", s.creds.Username, err)
	}
	if err := client.SendMail(s.creds.Username, rcpts, bytes.NewReader(raw)); err != nil {
		return fmt.Errorf("submitting message: %w", err)
	}
	return client.Quit()
}

// dialSMTPTLS connects over TLS: implicit TLS on the submissions port,
// STARTTLS everywhere else. There is no plaintext path. The emersion
// go-smtp dial helpers take no context, so the TCP connection (and TLS
// handshake on the implicit path) is dialed with DialContext and handed
// to smtp.NewClient/NewClientStartTLS. Once connected, the library's own
// CommandTimeout/SubmissionTimeout (5m/12m defaults) bound the session;
// cancellation of in-flight commands is covered by the watcher in
// SendMessage.
func dialSMTPTLS(ctx context.Context, srv serverConfig) (*smtp.Client, error) {
	host, port, err := resolveDialServer(srv, defaultSMTPPort)
	if err != nil {
		return nil, fmt.Errorf("stored smtp server: %w", err)
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	tlsCfg := tlsConfigFor(host)
	if smtpUsesImplicitTLS(port) {
		dialer := &tls.Dialer{
			NetDialer: &net.Dialer{Timeout: 30 * time.Second},
			Config:    tlsCfg,
		}
		conn, err := dialer.DialContext(ctx, "tcp", addr)
		if err != nil {
			return nil, fmt.Errorf("connecting to smtp server %s: %w", addr, err)
		}
		return smtp.NewClient(conn), nil
	}
	conn, err := (&net.Dialer{Timeout: 30 * time.Second}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("connecting to smtp server %s: %w", addr, err)
	}
	client, err := smtp.NewClientStartTLS(conn, tlsCfg)
	if err != nil {
		return nil, fmt.Errorf("starttls with smtp server %s: %w", addr, err)
	}
	return client, nil
}

// analyzeBodyStructure walks a BODYSTRUCTURE tree and returns the IMAP
// part path of the first inline text/plain part (nil when the message has
// none) plus the attachment metadata (filename, content type, and the
// server-declared size in octets) of every part with an attachment
// disposition — so attachment bodies are never fetched just to count them.
func analyzeBodyStructure(bs imap.BodyStructure) (bodyPath []int, attachments []Attachment) {
	bs.Walk(func(path []int, part imap.BodyStructure) bool {
		single, ok := part.(*imap.BodyStructureSinglePart)
		if !ok {
			return true // descend into multipart children
		}
		isAttachment := false
		if disp := part.Disposition(); disp != nil {
			isAttachment = strings.EqualFold(disp.Value, "attachment")
		}
		if isAttachment {
			attachments = append(attachments, Attachment{
				Filename:    single.Filename(),
				ContentType: single.MediaType(),
				Size:        int64(single.Size),
			})
			return true
		}
		if bodyPath == nil && single.MediaType() == "text/plain" {
			bodyPath = append([]int(nil), path...)
		}
		return true
	})
	return bodyPath, attachments
}

// parseHeaderBlock decodes a message's RFC 5322 header block (BODY[HEADER]
// bytes, which include the delimiting blank line) into the Message header
// fields.
func parseHeaderBlock(uid uint32, headerRaw []byte) (*Message, error) {
	mr, err := mail.CreateReader(bytes.NewReader(headerRaw))
	if err != nil {
		return nil, fmt.Errorf("parsing message header: %w", err)
	}
	msg := &Message{UID: uid, Headers: readHeaders(mr.Header)}
	if subject, err := mr.Header.Subject(); err == nil {
		msg.Subject = subject
	}
	if date, err := mr.Header.Date(); err == nil {
		msg.Date = date
	}
	msg.From = firstAddress(mr.Header, "From")
	msg.To = addressStrings(mr.Header, "To")
	return msg, nil
}

// parseBodyPart decodes one fetched body part: mimeRaw is the part's own
// MIME header (BODY[n.MIME], including its delimiting blank line) and
// bodyRaw the still-encoded part body (BODY[n]). Glued back together they
// parse as a single-part message, so the mail package applies the part's
// transfer encoding and charset exactly as it would inside the full
// message.
func parseBodyPart(mimeRaw, bodyRaw []byte) (string, error) {
	raw := make([]byte, 0, len(mimeRaw)+len(bodyRaw))
	raw = append(raw, mimeRaw...)
	raw = append(raw, bodyRaw...)
	mr, err := mail.CreateReader(bytes.NewReader(raw))
	if err != nil && !gomessage.IsUnknownCharset(err) {
		return "", fmt.Errorf("parsing body part: %w", err)
	}
	part, err := mr.NextPart()
	if errors.Is(err, io.EOF) {
		return "", nil // empty part
	}
	if err != nil && !gomessage.IsUnknownCharset(err) {
		return "", fmt.Errorf("reading body part: %w", err)
	}
	body, err := io.ReadAll(part.Body)
	if err != nil {
		return "", fmt.Errorf("reading body part: %w", err)
	}
	return string(body), nil
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

// readHeaders decodes every header field to UTF-8 text, preserving
// multi-valued headers (Received, ...) as repeated entries.
func readHeaders(h mail.Header) map[string][]string {
	out := make(map[string][]string)
	fields := h.Fields()
	for fields.Next() {
		text, err := fields.Text()
		if err != nil {
			text = fields.Value()
		}
		out[fields.Key()] = append(out[fields.Key()], text)
	}
	return out
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

// formatAddress renders "Name <addr>", or the bare address when there is
// no display name.
func formatAddress(name, addr string) string {
	if name != "" {
		return fmt.Sprintf("%s <%s>", name, addr)
	}
	return addr
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
		return formatAddress(a.Name, addr)
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
	return formatAddress(a.Name, a.Address)
}
