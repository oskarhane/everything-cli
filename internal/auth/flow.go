// Package auth runs the installed-app Google OAuth flow and keeps the
// per-account token cache fresh.
package auth

import (
	"context"
	"crypto/rand"
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

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// flowTimeout bounds the interactive wait for the browser callback.
const flowTimeout = 5 * time.Minute

// userinfoURL resolves the account email after the flow.
const userinfoURL = "https://www.googleapis.com/oauth2/v2/userinfo"

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

// newState returns the anti-CSRF state for the flow. Seam for tests.
var newState = func() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("google-cli-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// exchangeCode swaps an authorization code for tokens. Seam for tests.
var exchangeCode = func(ctx context.Context, conf *oauth2.Config, code string) (*oauth2.Token, error) {
	return conf.Exchange(ctx, code)
}

// fetchEmail resolves the account email from the userinfo endpoint. Seam for tests.
var fetchEmail = func(ctx context.Context, tok *oauth2.Token) (string, error) {
	resp, err := oauth2.NewClient(ctx, oauth2.StaticTokenSource(tok)).Get(userinfoURL)
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

// RunFlow performs the installed-app OAuth flow for the given scopes:
// parses the credentials file, starts a localhost listener on a random port
// as the redirect URI, prints the authorization URL (and tries to open a
// browser), waits for the code callback, exchanges it, and resolves the
// account email from the userinfo endpoint.
func RunFlow(credentialsPath string, scopes []string) (*oauth2.Token, string, error) {
	data, err := os.ReadFile(credentialsPath)
	if err != nil {
		return nil, "", fmt.Errorf("reading credentials %s: %w", credentialsPath, err)
	}
	conf, err := google.ConfigFromJSON(data, withEmailScope(scopes)...)
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

	state := newState()
	authURL := conf.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "consent"),
	)

	emit("Authorize google-cli by opening:\n\n%s\n\n", authURL)
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

	tok, err := exchangeCode(context.Background(), conf, code)
	if err != nil {
		return nil, "", fmt.Errorf("exchanging authorization code: %w", err)
	}
	email, err := fetchEmail(context.Background(), tok)
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

// withEmailScope appends the userinfo.email scope unless already granted,
// so RunFlow can always resolve the account email.
func withEmailScope(scopes []string) []string {
	out := make([]string, 0, len(scopes)+1)
	out = append(out, scopes...)
	for _, s := range out {
		if s == ScopeUserEmail {
			return out
		}
	}
	return append(out, ScopeUserEmail)
}
