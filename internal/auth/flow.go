// Package auth runs installed-app OAuth flows and keeps the per-account
// token cache fresh. The flow machinery is provider-general (RunFlowWith,
// TokenSourceWith): endpoints and the identity URL come from an OAuthProfile
// supplied by the provider, never from user-supplied files.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/spf13/afero"
	"golang.org/x/oauth2"
)

// flowTimeout bounds the interactive wait for the browser callback.
const flowTimeout = 5 * time.Minute

// networkTimeout bounds each network round trip the flow makes (the code
// exchange and the userinfo fetch) so a hung endpoint cannot stall the CLI
// indefinitely. Var so tests can shrink it.
var networkTimeout = 60 * time.Second

// flowOutput receives the authorization URL and flow status lines (stderr,
// so stdout stays data-clean).
var flowOutput io.Writer = os.Stderr

// openBrowser tries to open a URL in the user's browser. Seam for tests.
var openBrowser = func(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

// listenLoopback starts the local OAuth redirect listener on a random port.
// Seam for tests.
var listenLoopback = func() (net.Listener, error) {
	return net.Listen("tcp", "127.0.0.1:0")
}

// randRead is the random source for the state and PKCE secrets. Seam for
// tests, which stub a failing reader to force the error paths.
var randRead = rand.Read

// newState returns the anti-CSRF state for the flow. Seam for tests.
var newState = func() (string, error) {
	b := make([]byte, 16)
	if _, err := randRead(b); err != nil {
		return "", fmt.Errorf("generating state: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// pkceVerifierBytes is the entropy fed into the PKCE verifier; hex encoding
// doubles it to 128 characters, within RFC 7636's 43–128 char range.
const pkceVerifierBytes = 64

// newPKCE returns a fresh PKCE verifier (hex, RFC 7636 unreserved charset)
// and its S256 challenge.
func newPKCE() (verifier, challenge string, err error) {
	b := make([]byte, pkceVerifierBytes)
	if _, err := randRead(b); err != nil {
		return "", "", fmt.Errorf("generating PKCE verifier: %w", err)
	}
	verifier = hex.EncodeToString(b)
	sum := sha256.Sum256([]byte(verifier))
	return verifier, base64.RawURLEncoding.EncodeToString(sum[:]), nil
}

// exchangeCode swaps an authorization code for tokens, presenting the PKCE
// verifier generated with the code_challenge sent on the auth URL. Seam for
// tests.
var exchangeCode = func(ctx context.Context, conf *oauth2.Config, code, verifier string) (*oauth2.Token, error) {
	return conf.Exchange(ctx, code, oauth2.SetAuthURLParam("code_verifier", verifier))
}

// fetchEmail resolves the account email from the provider's userinfo
// endpoint. Seam for tests.
var fetchEmail = func(ctx context.Context, url string, tok *oauth2.Token) (string, error) {
	resp, err := oauth2.NewClient(ctx, oauth2.StaticTokenSource(tok)).Get(url)
	if err != nil {
		return "", fmt.Errorf("calling userinfo endpoint: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("userinfo endpoint returned %s", resp.Status)
	}
	var out struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("decoding userinfo response: %w", err)
	}
	if out.Email == "" {
		return "", errors.New("userinfo response carried no email")
	}
	return out.Email, nil
}

// RunFlow performs the installed-app OAuth flow for Google: it is
// RunFlowWith with the GoogleOAuth profile.
func RunFlow(fs afero.Fs, credentialsPath string, scopes []string) (*oauth2.Token, string, error) {
	return RunFlowWith(fs, credentialsPath, scopes, GoogleOAuth)
}

// RunFlowWith performs the installed-app OAuth flow for any provider
// described by profile: parses the credentials file (endpoints pinned to
// profile's, never the file's), starts a localhost listener on a random
// port as the redirect URI, prints the authorization URL (and tries to open
// a browser), waits for the code callback, exchanges it, and resolves the
// account email from the profile's userinfo endpoint.
func RunFlowWith(fs afero.Fs, credentialsPath string, scopes []string, profile OAuthProfile) (*oauth2.Token, string, error) {
	data, err := afero.ReadFile(fs, credentialsPath)
	if err != nil {
		return nil, "", fmt.Errorf("reading credentials %s: %w", credentialsPath, err)
	}
	conf, err := credentialsConfigFor(profile, data, ensureScope(scopes, profile.EmailScope)...)
	if err != nil {
		return nil, "", fmt.Errorf("parsing credentials %s: %w", credentialsPath, err)
	}

	ln, err := listenLoopback()
	if err != nil {
		return nil, "", fmt.Errorf("starting local redirect listener: %w", err)
	}
	defer func() { _ = ln.Close() }()

	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		return nil, "", fmt.Errorf("local redirect listener address %v is not TCP", ln.Addr())
	}
	conf.RedirectURL = fmt.Sprintf("http://localhost:%d", addr.Port)

	state, err := newState()
	if err != nil {
		return nil, "", err
	}
	verifier, challenge, err := newPKCE()
	if err != nil {
		return nil, "", err
	}
	authURL := conf.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "consent"),
		oauth2.SetAuthURLParam("code_challenge", challenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)

	emit("Authorize %s by opening:\n\n%s\n\n", profile.Name, authURL)
	if err := openBrowser(authURL); err != nil {
		emit("Could not open a browser; open the URL above manually.\n")
	}

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		code, err := callbackCode(r, state)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			flush(w)
			select {
			case errCh <- err:
			default:
			}
			return
		}
		writeCallbackPage(w)
		select {
		case codeCh <- code:
		default:
		}
	})}
	srvErr := make(chan error, 1)
	go func() { srvErr <- srv.Serve(ln) }()
	defer func() { _ = srv.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), flowTimeout)
	defer cancel()

	var code string
	select {
	case code = <-codeCh:
	case err := <-errCh:
		return nil, "", fmt.Errorf("authorization failed: %w", err)
	case err := <-srvErr:
		return nil, "", fmt.Errorf("redirect listener stopped: %v", err)
	case <-ctx.Done():
		return nil, "", fmt.Errorf("timed out after %s waiting for authorization at %s",
			flowTimeout, conf.RedirectURL)
	}

	if err := srv.Close(); err != nil {
		return nil, "", fmt.Errorf("closing redirect listener: %w", err)
	}
	if err := <-srvErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return nil, "", fmt.Errorf("redirect listener: %w", err)
	}

	exCtx, cancelExchange := context.WithTimeout(context.Background(), networkTimeout)
	defer cancelExchange()
	tok, err := exchangeCode(exCtx, conf, code, verifier)
	if err != nil {
		return nil, "", fmt.Errorf("exchanging authorization code: %w", err)
	}
	// Mint point: register the fresh token's secrets for redaction before
	// anything else can render them.
	registerTokenSecrets(tok)
	emailCtx, cancelEmail := context.WithTimeout(context.Background(), networkTimeout)
	defer cancelEmail()
	email, err := fetchEmail(emailCtx, profile.UserinfoURL, tok)
	if err != nil {
		return nil, "", err
	}
	return tok, email, nil
}

// emit writes a best-effort status line to flowOutput: flow output must
// never fail the authorization.
func emit(format string, args ...any) {
	_, _ = fmt.Fprintf(flowOutput, format, args...)
}

// writeCallbackPage renders the browser-facing success page and flushes it,
// so the page reaches the browser before the flow tears the listener down.
func writeCallbackPage(w http.ResponseWriter) {
	_, _ = io.WriteString(w, "Authorization complete. You can close this window.\n")
	flush(w)
}

// flush pushes an already-written HTTP response out to the client.
func flush(w http.ResponseWriter) {
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// callbackCode extracts and validates the code from the OAuth redirect.
func callbackCode(r *http.Request, state string) (string, error) {
	q := r.URL.Query()
	if e := q.Get("error"); e != "" {
		return "", fmt.Errorf("authorization was rejected: %s", e)
	}
	if got := q.Get("state"); got != state {
		return "", errors.New("state mismatch in OAuth redirect")
	}
	code := q.Get("code")
	if code == "" {
		return "", errors.New("OAuth redirect carried no code")
	}
	return code, nil
}

// ensureScope appends scope to scopes unless already granted, so a flow can
// always guarantee the scope its identity resolution depends on.
func ensureScope(scopes []string, scope string) []string {
	out := make([]string, 0, len(scopes)+1)
	out = append(out, scopes...)
	for _, s := range out {
		if s == scope {
			return out
		}
	}
	return append(out, scope)
}
