package service

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"golang.org/x/oauth2"
)

// TestAuthenticatedClientCarriesAuth pins the regression where the timeout
// client was passed bare via WithHTTPClient, which takes precedence over
// WithTokenSource — every Calendar call went out with no Authorization
// header. The client that New builds must carry the token source in its
// transport.
func TestAuthenticatedClientCarriesAuth(t *testing.T) {
	var (
		mu  sync.Mutex
		hdr string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hdr = r.Header.Get("Authorization")
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test-token", TokenType: "Bearer"})
	client := authenticatedClient(ts, apiTimeout)

	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("request through authenticated client: %v", err)
	}
	if err := resp.Body.Close(); err != nil {
		t.Fatalf("closing response body: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if hdr != "Bearer test-token" {
		t.Fatalf("Authorization header = %q, want %q — the client New builds is sending unauthenticated requests", hdr, "Bearer test-token")
	}
}
