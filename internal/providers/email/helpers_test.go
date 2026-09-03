package email

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/config"
	"github.com/oskarhane/everything-cli/internal/output"
)

// TestMain pins format auto-detection off: the host machine may run this
// suite inside an agent harness or a TTY, and neither may flip expectations.
// Tests always pass an explicit --format when the format matters.
func TestMain(m *testing.M) {
	output.IsAgent = func() bool { return false }
	output.StdoutIsTerminal = func() bool { return false }
	os.Exit(m.Run())
}

// newEmailEnv is the one shared hermetic env for every email test: an
// in-memory FS, a pinned config dir, and the REAL provider tree
// (Provider{}.NewCmd) mounted on a fresh root command whose stdout is
// captured. Leaf tests always run against the production wiring — never a
// bespoke mount. Tests never touch the real config dir. The password env
// var is blanked so a host EMAIL_PASSWORD cannot leak into a test.
func newEmailEnv(t *testing.T) (*app.Config, *cobra.Command, *bytes.Buffer) {
	t.Helper()
	t.Setenv(config.EnvConfigDir, "/config")
	t.Setenv(passwordEnvVar, "")
	cfg := &app.Config{Fs: afero.NewMemMapFs()}
	root := app.NewRootCommand(cfg)
	root.AddCommand(Provider{}.NewCmd(cfg))
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(io.Discard)
	return cfg, root, out
}

// execute runs the command tree with args and returns the captured stdout
// and the command's error. Cobra's usage/error output goes to io.Discard so
// out holds only command output.
func execute(t *testing.T, root *cobra.Command, out *bytes.Buffer, args ...string) (string, error) {
	t.Helper()
	out.Reset()
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

// stubDial swaps a dial seam (dialMail or dialSendMail) for the test's
// lifetime so the leaf resolves svc instead of touching the network;
// restored via t.Cleanup.
func stubDial(t *testing.T, seam *func(context.Context, *app.Config) (MailService, error), svc MailService, err error) {
	t.Helper()
	saved := *seam
	*seam = func(context.Context, *app.Config) (MailService, error) { return svc, err }
	t.Cleanup(func() { *seam = saved })
}

// stubTLSRoots points the adapter's TLS config seam at the test CA for the
// test's lifetime, keeping full verification against the loopback servers.
func stubTLSRoots(t *testing.T, roots *x509.CertPool) {
	t.Helper()
	saved := tlsConfigFor
	tlsConfigFor = func(host string) *tls.Config {
		cfg := saved(host)
		cfg.RootCAs = roots
		return cfg
	}
	t.Cleanup(func() { tlsConfigFor = saved })
}

// fakeMailService is the one shared MailService fake for the leaf tests.
// Each concern is a func field: a test sets only the field its leaf
// consumes, and a call through any unset method fails the test with an
// "unexpected call" error, proving the leaf stays inside its narrowed
// surface. The got* fields record call arguments for flag-plumbing
// assertions; closed proves the leaf's deferred Close ran.
type fakeMailService struct {
	listMailboxesFn func(context.Context) ([]string, error)
	listEnvelopesFn func(context.Context, string, int) ([]Envelope, error)
	getMessageFn    func(context.Context, string, uint32) (*Message, error)
	sendMessageFn   func(context.Context, SendInput) error

	gotMailbox string
	gotLimit   int
	gotUID     uint32
	gotSend    SendInput
	gotBody    string

	closed bool
}

func (f *fakeMailService) ListMailboxes(ctx context.Context) ([]string, error) {
	if f.listMailboxesFn == nil {
		return nil, errors.New("fakeMailService: unexpected ListMailboxes call")
	}
	return f.listMailboxesFn(ctx)
}

func (f *fakeMailService) ListEnvelopes(ctx context.Context, mailbox string, limit int) ([]Envelope, error) {
	if f.listEnvelopesFn == nil {
		return nil, errors.New("fakeMailService: unexpected ListEnvelopes call")
	}
	f.gotMailbox, f.gotLimit = mailbox, limit
	return f.listEnvelopesFn(ctx, mailbox, limit)
}

func (f *fakeMailService) GetMessage(ctx context.Context, mailbox string, uid uint32) (*Message, error) {
	if f.getMessageFn == nil {
		return nil, errors.New("fakeMailService: unexpected GetMessage call")
	}
	f.gotMailbox, f.gotUID = mailbox, uid
	return f.getMessageFn(ctx, mailbox, uid)
}

func (f *fakeMailService) SendMessage(ctx context.Context, in SendInput) error {
	if f.sendMessageFn == nil {
		return errors.New("fakeMailService: unexpected SendMessage call")
	}
	return f.sendMessageFn(ctx, in)
}

func (f *fakeMailService) Close() error {
	f.closed = true
	return nil
}

// mailboxFake serves names/err from ListMailboxes.
func mailboxFake(names []string, err error) *fakeMailService {
	return &fakeMailService{listMailboxesFn: func(context.Context) ([]string, error) {
		return names, err
	}}
}

// listFake serves envelopes/err from ListEnvelopes.
func listFake(envelopes []Envelope, err error) *fakeMailService {
	return &fakeMailService{listEnvelopesFn: func(context.Context, string, int) ([]Envelope, error) {
		return envelopes, err
	}}
}

// getFake serves msg/err from GetMessage.
func getFake(msg *Message, err error) *fakeMailService {
	return &fakeMailService{getMessageFn: func(context.Context, string, uint32) (*Message, error) {
		return msg, err
	}}
}

// sendFake captures the SendInput — eagerly draining the single-use body
// reader, since the leaf hands over a streaming reader — and returns
// sendErr.
func sendFake(sendErr error) *fakeMailService {
	f := &fakeMailService{}
	f.sendMessageFn = func(_ context.Context, in SendInput) error {
		f.gotSend = in
		if in.Body != nil {
			content, err := io.ReadAll(in.Body)
			if err == nil {
				f.gotBody = string(content)
			}
		}
		return sendErr
	}
	return f
}

// newStore returns the store the commands build: the injected in-memory FS
// with the same config-dir resolution as production.
func newStore(t *testing.T, cfg *app.Config) *config.Store {
	t.Helper()
	store, err := config.NewStore(cfg.Fs, "")
	require.NoError(t, err)
	return store
}

// stubCapture swaps the getenv/prompt seams for the test's lifetime so
// capture-order tests run hermetically — never against the real
// environment or a terminal.
func stubCapture(t *testing.T, getenvFn func(string) string, promptFn func() (string, error)) {
	t.Helper()
	savedGetenv, savedPrompt := getenv, prompt
	if getenvFn != nil {
		getenv = getenvFn
	}
	if promptFn != nil {
		prompt = promptFn
	}
	t.Cleanup(func() { getenv, prompt = savedGetenv, savedPrompt })
}

// seedAccount persists an email account directly through addAccount so
// read/remove tests start from a known state without a prompt. The
// password is distinctive per test so assertions can prove it never
// reaches output.
func seedAccount(t *testing.T, cfg *app.Config, name, password string) {
	t.Helper()
	_, err := addAccount(newStore(t, cfg), addOptions{
		Name:     name,
		Username: name + "@example.com",
		Password: password,
		IMAPHost: "imap.example.com",
		IMAPPort: defaultIMAPPort,
		SMTPHost: "smtp.example.com",
		SMTPPort: defaultSMTPPort,
	})
	require.NoError(t, err)
}
