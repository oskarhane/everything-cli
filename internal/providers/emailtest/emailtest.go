// Package emailtest hosts the in-process IMAP/SMTP servers backing the
// email provider's hermetic adapter tests. It lives outside
// internal/providers/email so the emersion imports stay confined to
// adapter.go in that package (the adapter is the only production file
// allowed to talk to the mail libraries).
package emailtest

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"io"
	"math/big"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	imap "github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapserver"
	"github.com/emersion/go-imap/v2/imapserver/imapmemserver"
	"github.com/emersion/go-sasl"
	smtp "github.com/emersion/go-smtp"
)

// SeedMessage is one raw RFC 5322 message to append to a test mailbox,
// with the IMAP flags and internal date it should carry.
type SeedMessage struct {
	Raw   string
	Flags []string
	Time  time.Time
}

// TLSConfigs mints a self-signed loopback certificate: the server config
// presents it, and the returned pool lets the adapter's TLS config seam
// trust it. Real hosts always verify against the system roots — this
// exists only so tests exercise the same TLS-only code paths on 127.0.0.1.
func TLSConfigs(t *testing.T) (server *tls.Config, roots *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generating test key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "everything-cli emailtest"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:              []string{"localhost"},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("creating test certificate: %v", err)
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parsing test certificate: %v", err)
	}
	roots = x509.NewCertPool()
	roots.AddCert(leaf)
	return &tls.Config{
		Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}},
		MinVersion:   tls.VersionTLS12,
	}, roots
}

// StartIMAP starts an in-process imapmemserver over implicit TLS on a
// loopback port, with one user and the given mailboxes pre-seeded. It
// returns the dial host/port and the root pool the client must trust.
func StartIMAP(t *testing.T, username, password string, mailboxes map[string][]SeedMessage) (host string, port int, roots *x509.CertPool) {
	t.Helper()
	serverTLS, pool := TLSConfigs(t)
	server := newIMAPServer(t, username, password, mailboxes, serverTLS)
	ln, err := tls.Listen("tcp", "127.0.0.1:0", serverTLS)
	if err != nil {
		t.Fatalf("listening: %v", err)
	}
	serveIMAP(t, server, ln)
	host, port = hostPort(t, ln.Addr())
	return host, port, pool
}

// StartIMAPStartTLS starts the same server on a PLAINTEXT listener with
// TLSConfig set, so it advertises STARTTLS pre-auth and upgrades the
// connection on demand — the transport the adapter picks for non-993
// ports. Authentication still requires the upgraded (TLS) connection:
// the server never enables InsecureAuth.
func StartIMAPStartTLS(t *testing.T, username, password string, mailboxes map[string][]SeedMessage) (host string, port int, roots *x509.CertPool) {
	t.Helper()
	serverTLS, pool := TLSConfigs(t)
	server := newIMAPServer(t, username, password, mailboxes, serverTLS)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listening: %v", err)
	}
	serveIMAP(t, server, ln)
	host, port = hostPort(t, ln.Addr())
	return host, port, pool
}

// newIMAPServer builds the seeded in-memory server both transports share.
func newIMAPServer(t *testing.T, username, password string, mailboxes map[string][]SeedMessage, serverTLS *tls.Config) *imapserver.Server {
	t.Helper()
	mem := imapmemserver.New()
	user := imapmemserver.NewUser(username, password)
	for name, msgs := range mailboxes {
		if err := user.Create(name, nil); err != nil {
			t.Fatalf("creating mailbox %q: %v", name, err)
		}
		for _, m := range msgs {
			opts := &imap.AppendOptions{Time: m.Time}
			for _, f := range m.Flags {
				opts.Flags = append(opts.Flags, imap.Flag(f))
			}
			if _, err := user.Append(name, newLiteral(m.Raw), opts); err != nil {
				t.Fatalf("seeding message in %q: %v", name, err)
			}
		}
	}
	mem.AddUser(user)

	return imapserver.New(&imapserver.Options{
		NewSession: func(*imapserver.Conn) (imapserver.Session, *imapserver.GreetingData, error) {
			return mem.NewSession(), nil, nil
		},
		TLSConfig: serverTLS,
		Caps: imap.CapSet{
			imap.CapIMAP4rev1: {},
			imap.CapIMAP4rev2: {},
		},
	})
}

// serveIMAP runs the server on ln until test cleanup.
func serveIMAP(t *testing.T, server *imapserver.Server, ln net.Listener) {
	t.Helper()
	go func() { _ = server.Serve(ln) }()
	t.Cleanup(func() { _ = server.Close() })
}

// DeliveredMessage is one message captured by the test SMTP server.
type DeliveredMessage struct {
	From string
	To   []string
	Data []byte
}

// SMTPServer is an in-process submission server that captures deliveries.
type SMTPServer struct {
	Host  string
	Port  int
	Roots *x509.CertPool

	mu       sync.Mutex
	messages []DeliveredMessage
}

// Messages returns every message the server has accepted so far.
func (s *SMTPServer) Messages() []DeliveredMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]DeliveredMessage(nil), s.messages...)
}

// StartSMTP starts an in-process go-smtp server on loopback that requires
// PLAIN auth with username/password. implicitTLS selects the transport the
// adapter picks for the submissions port (465): a TLS listener instead of
// STARTTLS negotiation.
func StartSMTP(t *testing.T, username, password string, implicitTLS bool) *SMTPServer {
	t.Helper()
	serverTLS, roots := TLSConfigs(t)

	srv := &SMTPServer{Roots: roots}
	backend := smtp.BackendFunc(func(*smtp.Conn) (smtp.Session, error) {
		return &testSession{srv: srv, username: username, password: password}, nil
	})
	server := smtp.NewServer(backend)
	server.TLSConfig = serverTLS
	server.AllowInsecureAuth = false

	var (
		ln  net.Listener
		err error
	)
	if implicitTLS {
		ln, err = tls.Listen("tcp", "127.0.0.1:0", serverTLS)
	} else {
		ln, err = net.Listen("tcp", "127.0.0.1:0")
	}
	if err != nil {
		t.Fatalf("listening: %v", err)
	}
	go func() { _ = server.Serve(ln) }()
	t.Cleanup(func() { _ = server.Close() })

	srv.Host, srv.Port = hostPort(t, ln.Addr())
	return srv
}

// testSession is the SMTP server session: it authenticates with the fixed
// credentials and captures every accepted message.
type testSession struct {
	srv      *SMTPServer
	username string
	password string
	msg      DeliveredMessage
}

var _ smtp.AuthSession = (*testSession)(nil)

func (s *testSession) AuthMechanisms() []string { return []string{sasl.Plain} }

func (s *testSession) Auth(mech string) (sasl.Server, error) {
	return sasl.NewPlainServer(func(identity, username, password string) error {
		if identity != "" && identity != username {
			return smtp.ErrAuthFailed
		}
		if username != s.username || password != s.password {
			return smtp.ErrAuthFailed
		}
		return nil
	}), nil
}

func (s *testSession) Mail(from string, _ *smtp.MailOptions) error {
	s.msg = DeliveredMessage{From: from}
	return nil
}

func (s *testSession) Rcpt(to string, _ *smtp.RcptOptions) error {
	s.msg.To = append(s.msg.To, to)
	return nil
}

func (s *testSession) Data(r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	s.msg.Data = data
	s.srv.mu.Lock()
	s.srv.messages = append(s.srv.messages, s.msg)
	s.srv.mu.Unlock()
	return nil
}

func (s *testSession) Reset() { s.msg = DeliveredMessage{} }

func (s *testSession) Logout() error { return nil }

// literal adapts a string to imap.LiteralReader for seeding.
type literal struct {
	*strings.Reader
}

func newLiteral(s string) *literal {
	return &literal{Reader: strings.NewReader(s)}
}

func (l *literal) Size() int64 { return l.Reader.Size() }

func hostPort(t *testing.T, addr net.Addr) (string, int) {
	t.Helper()
	host, portStr, err := net.SplitHostPort(addr.String())
	if err != nil {
		t.Fatalf("splitting %q: %v", addr.String(), err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parsing port %q: %v", portStr, err)
	}
	return host, port
}
