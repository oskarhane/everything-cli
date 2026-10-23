package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// fileServerConfig tunes one hermetic file-storage endpoint.
type fileServerConfig struct {
	content    []byte
	status     int    // 0 => 200
	redirectTo string // 302 here instead of serving
}

// fileRecorder records the Authorization header of every request the file
// endpoint received.
type fileRecorder struct {
	mu   sync.Mutex
	auth []string
}

func (r *fileRecorder) record(auth string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.auth = append(r.auth, auth)
}

func (r *fileRecorder) requests() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.auth...)
}

// newFileServer serves cfg's bytes, status, or redirect at every path and
// records every request it receives, so tests can prove both that the
// authenticated client reached it and that a refused URL never did.
func newFileServer(t *testing.T, cfg fileServerConfig) (*httptest.Server, *fileRecorder) {
	t.Helper()
	rec := &fileRecorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.record(r.Header.Get("Authorization"))
		if cfg.redirectTo != "" {
			http.Redirect(w, r, cfg.redirectTo, http.StatusFound)
			return
		}
		if cfg.status != 0 && cfg.status != http.StatusOK {
			http.Error(w, "boom", cfg.status)
			return
		}
		_, _ = w.Write(cfg.content)
	}))
	t.Cleanup(srv.Close)
	return srv, rec
}

// allowFileHost stubs the production allowlist to permit exactly the test
// server's host (plain http on a loopback port) for the test's lifetime,
// restoring the production policy after.
func allowFileHost(t *testing.T, srv *httptest.Server) {
	t.Helper()
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	saved := validateFileURL
	validateFileURL = func(v *url.URL) bool { return v.Host == u.Host }
	t.Cleanup(func() { validateFileURL = saved })
}

// TestDownloadFileStreamsBytes: a 200 answer streams to w byte-for-byte,
// control bytes included, and the request carries the authenticated
// client's raw API key (no Bearer prefix), like every other linear call.
func TestDownloadFileStreamsBytes(t *testing.T) {
	content := []byte("a\x00b\x07c\xff")
	srv, rec := newFileServer(t, fileServerConfig{content: content})
	allowFileHost(t, srv)
	svc := newTestService(srv)

	var buf bytes.Buffer
	require.NoError(t, svc.DownloadFileTo(context.Background(), srv.URL+"/file-uuid", &buf))

	require.Equal(t, content, buf.Bytes())
	require.Equal(t, []string{"test-key-123"}, rec.requests())
}

// TestDownloadFileStreamsInChunks: a large body reaches w through several
// small writes (io.Copy's fixed buffer), proving the body is streamed, not
// materialized before writing.
func TestDownloadFileStreamsInChunks(t *testing.T) {
	content := bytes.Repeat([]byte{0xAB}, 128*1024)
	srv, _ := newFileServer(t, fileServerConfig{content: content})
	allowFileHost(t, srv)
	svc := newTestService(srv)

	cw := &chunkWriter{}
	require.NoError(t, svc.DownloadFileTo(context.Background(), srv.URL+"/file-uuid", cw))

	require.Equal(t, int64(len(content)), cw.total)
	require.Greater(t, len(cw.sizes), 1, "the body must stream in chunks, not one buffered write")
}

// chunkWriter counts writes and their sizes without keeping the bytes.
type chunkWriter struct {
	total int64
	sizes []int
}

func (w *chunkWriter) Write(p []byte) (int, error) {
	w.total += int64(len(p))
	w.sizes = append(w.sizes, len(p))
	return len(p), nil
}

// TestDownloadFileNon200Surfaces: a non-200 answer becomes the same status
// plus capped body error every other linear call surfaces.
func TestDownloadFileNon200Surfaces(t *testing.T) {
	srv, _ := newFileServer(t, fileServerConfig{status: http.StatusBadGateway})
	allowFileHost(t, srv)
	svc := newTestService(srv)

	err := svc.DownloadFileTo(context.Background(), srv.URL+"/file-uuid", io.Discard)

	require.ErrorContains(t, err, "linear API returned 502")
	require.ErrorContains(t, err, "boom")
}

// TestDownloadFileAllowlistRejects: URLs outside the allowlist fail fast —
// the error names the allowlist and no request is issued, so the account's
// Authorization header can never leave Linear's own storage host.
func TestDownloadFileAllowlistRejects(t *testing.T) {
	srv, rec := newFileServer(t, fileServerConfig{content: []byte("bytes")})
	svc := newTestService(srv)
	// The production allowlist stays in force; the live server stands in for
	// the attacker, so a leaked request would be recorded.
	host := urlToHost(t, srv.URL)

	tests := []struct {
		name string
		url  string
	}{
		{"http scheme", "http://" + host + "/file-uuid"},
		{"other host", "https://files.example.com/file-uuid"},
		{"subdomain of the allowed host", "https://evil.uploads.linear.app/file-uuid"},
		{"graphql endpoint host", "https://api.linear.app/graphql"},
		{"unparseable", "://bad"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.DownloadFileTo(context.Background(), tt.url, io.Discard)
			require.Error(t, err)
			require.Contains(t, err.Error(), "linear file download refused URL")
			require.Contains(t, err.Error(), "only https://uploads.linear.app")
		})
	}
	require.Empty(t, rec.requests(), "no request may be issued for a refused URL")
}

// urlToHost returns the host:port of a URL string.
func urlToHost(t *testing.T, raw string) string {
	t.Helper()
	u, err := url.Parse(raw)
	require.NoError(t, err)
	return u.Host
}

// TestDownloadFileRefusesCrossHostRedirect: a redirect off the download host
// is refused, and the redirect target never receives a request — the
// transport would re-stamp Authorization after Go's client strips it, so the
// guard must stop the follow before it happens.
func TestDownloadFileRefusesCrossHostRedirect(t *testing.T) {
	evil, evilRec := newFileServer(t, fileServerConfig{content: []byte("stolen")})
	srv, _ := newFileServer(t, fileServerConfig{redirectTo: evil.URL + "/stolen"})
	allowFileHost(t, srv)
	svc := newTestService(srv)

	err := svc.DownloadFileTo(context.Background(), srv.URL+"/file-uuid", io.Discard)

	require.Error(t, err)
	require.Contains(t, err.Error(), "cross-host redirect")
	require.Empty(t, evilRec.requests(), "the redirect target must never receive a request")
}

// TestDownloadFileErrorHidesURL: a connection failure surfaces the cause but
// never the URL string — wire-resolved URLs carry signature query params
// that must not reach output (url.Error embeds it; the error is unwrapped).
func TestDownloadFileErrorHidesURL(t *testing.T) {
	dead, _ := newFileServer(t, fileServerConfig{content: []byte("bytes")})
	deadURL := dead.URL + "/file-uuid"
	dead.Close()

	// Permit the (now dead) loopback host so the failure is a dial error, not
	// an allowlist rejection.
	saved := validateFileURL
	validateFileURL = func(*url.URL) bool { return true }
	t.Cleanup(func() { validateFileURL = saved })

	svc := newTestService(dead)

	err := svc.DownloadFileTo(context.Background(), deadURL, io.Discard)

	require.Error(t, err)
	// The "calling linear file download" prefix proves the allowlist let the
	// request through to a real dial.
	require.Contains(t, err.Error(), "calling linear file download")
	require.NotContains(t, err.Error(), deadURL)
}
