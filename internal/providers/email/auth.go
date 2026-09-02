package email

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/oskarhane/everything-cli/internal/auth"
	"github.com/oskarhane/everything-cli/internal/config"
)

// passwordEnvVar is consulted for the password when --password is empty.
const passwordEnvVar = "EMAIL_PASSWORD"

// Default ports: implicit TLS for IMAP, submission (STARTTLS) for SMTP.
const (
	defaultIMAPPort = 993
	defaultSMTPPort = 587
)

// getenv and prompt are seams for hermetic tests; production wiring gets
// the defaults (os.Getenv and a hidden terminal prompt).
var (
	getenv = os.Getenv
	prompt = promptHidden
)

// serverConfig pins one endpoint of the provider-shaped auth payload.
type serverConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

// credentials is the provider-shaped JSON stored in Account.Auth, opaque
// to the store. The password is a secret on par with an OAuth refresh
// token: it is captured without echo, never printed, and registered with
// the redaction registry at the capture/read point (AGENTS.md rule).
type credentials struct {
	Username string       `json:"username"`
	Password string       `json:"password"`
	IMAP     serverConfig `json:"imap"`
	SMTP     serverConfig `json:"smtp"`
}

// addOptions carries the `email account add` inputs.
type addOptions struct {
	Name     string
	Username string
	Password string // flag value; empty = EMAIL_PASSWORD, then a hidden prompt
	IMAPHost string
	IMAPPort int
	SMTPHost string
	SMTPPort int
}

// addAccount validates opts, captures the password, and persists the
// account under the email store directory. The password is registered for
// redaction immediately at capture, before anything could print it.
func addAccount(store *config.Store, opts addOptions) (*config.Account, error) {
	if err := opts.validate(); err != nil {
		return nil, err
	}
	password, err := capturePassword(opts.Password)
	if err != nil {
		return nil, err
	}
	// Mint point (AGENTS.md rule): register before any output path exists.
	auth.RegisterSecret(password)
	payload, err := json.Marshal(credentials{
		Username: strings.TrimSpace(opts.Username),
		Password: password,
		IMAP:     serverConfig{Host: strings.TrimSpace(opts.IMAPHost), Port: opts.IMAPPort},
		SMTP:     serverConfig{Host: strings.TrimSpace(opts.SMTPHost), Port: opts.SMTPPort},
	})
	if err != nil {
		return nil, fmt.Errorf("encoding auth payload: %w", err)
	}
	acct := &config.Account{
		Name:     opts.Name,
		Provider: providerID,
		Auth:     payload,
	}
	if err := store.Save(acct); err != nil {
		return nil, err
	}
	return store.GetProvider(providerID, acct.Name)
}

// validate rejects an incomplete endpoint set before any password is
// captured, so a missing flag fails fast instead of after a prompt.
func (o addOptions) validate() error {
	switch {
	case strings.TrimSpace(o.Username) == "":
		return errors.New("--username is required")
	case strings.TrimSpace(o.IMAPHost) == "":
		return errors.New("--imap-host is required")
	case strings.TrimSpace(o.SMTPHost) == "":
		return errors.New("--smtp-host is required")
	}
	if err := validPort("--imap-port", o.IMAPPort); err != nil {
		return err
	}
	return validPort("--smtp-port", o.SMTPPort)
}

func validPort(flag string, port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("%s must be between 1 and 65535, got %d", flag, port)
	}
	return nil
}

// capturePassword resolves the password from the flag value, then
// EMAIL_PASSWORD, then the hidden prompt.
func capturePassword(flagValue string) (string, error) {
	password := strings.TrimSpace(flagValue)
	if password == "" {
		password = strings.TrimSpace(getenv(passwordEnvVar))
	}
	if password == "" {
		prompted, err := prompt()
		if err != nil {
			return "", err
		}
		password = strings.TrimSpace(prompted)
	}
	if password != "" {
		return password, nil
	}
	return "", fmt.Errorf("no password: pass --password, set %s, or enter it at the hidden prompt", passwordEnvVar)
}

// promptHidden reads the password from the terminal without echo. The
// prompt goes to stderr so stdout stays machine-readable; the typed
// password is never written anywhere.
func promptHidden() (string, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return "", fmt.Errorf("no password and stdin is not a terminal: pass --password or set %s", passwordEnvVar)
	}
	_, _ = fmt.Fprint(os.Stderr, "Password: ")
	raw, err := term.ReadPassword(fd)
	_, _ = fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("reading password: %w", err)
	}
	return string(raw), nil
}

// loadCredentials parses the account's stored auth payload. The password
// read from disk is re-registered for redaction (read point, AGENTS.md
// rule): a secret restored from disk must be scrubbed from output too.
func loadCredentials(acct *config.Account) (*credentials, error) {
	if acct == nil {
		return nil, errors.New("no account")
	}
	var creds credentials
	if err := json.Unmarshal(acct.Auth, &creds); err != nil {
		return nil, fmt.Errorf("parsing account %q auth: %w", acct.Name, err)
	}
	if creds.Password == "" {
		return nil, fmt.Errorf("account %q holds no password", acct.Name)
	}
	auth.RegisterSecret(creds.Password)
	return &creds, nil
}
