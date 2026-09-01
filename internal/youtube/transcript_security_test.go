package youtube

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsAllowedTranscriptHost(t *testing.T) {
	allowed := []string{
		"youtube.com",
		"www.youtube.com",
		"m.youtube.com",
		"googlevideo.com",
		"rr1---sn-abc123.googlevideo.com",
		"YouTube.COM",
	}
	for _, host := range allowed {
		assert.True(t, isAllowedTranscriptHost(host), "host %q should be allowed", host)
	}

	rejected := []string{
		"",
		"127.0.0.1",
		"localhost",
		"evil.example.com",
		"notyoutube.com",
		"youtube.com.evil.com",
		"googlevideo.com.evil.com",
		"notgooglevideo.com",
	}
	for _, host := range rejected {
		assert.False(t, isAllowedTranscriptHost(host), "host %q should be rejected", host)
	}
}

// TestTranscriptRejectsOffHostTrackURL runs with the host allowlist seam in
// its default state: a track URL outside the allowlist must be rejected
// before any request is built.
func TestTranscriptRejectsOffHostTrackURL(t *testing.T) {
	_, err := NewClient().Transcript(context.Background(), "https://evil.example.com/tt")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "evil.example.com")
}

// TestTranscriptRedirectOffHostRejected lets the allowlist cover the
// httptest loopback host, then has that server 302 to an off-host URL. The
// redirect must be refused by name and the off-host target never fetched.
func TestTranscriptRedirectOffHostRejected(t *testing.T) {
	var evilHits int32
	evil := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&evilHits, 1)
		_, _ = w.Write([]byte(msXML))
	}))
	t.Cleanup(evil.Close)

	// The evil URL must differ from the allowed host in hostname, not just
	// port: both httptest servers live on 127.0.0.1, so reach the off-host
	// server via "localhost".
	evilURL, err := url.Parse(evil.URL)
	require.NoError(t, err)
	evilURL.Host = "localhost:" + evilURL.Port()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, evilURL.String(), http.StatusFound)
	}))
	t.Cleanup(srv.Close)
	allowLoopbackTranscriptHost(t)

	_, err = NewClient().Transcript(context.Background(), srv.URL+"/timedtext")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "localhost")
	assert.Equal(t, int32(0), atomic.LoadInt32(&evilHits), "off-host redirect target must never be fetched")
}

// TestTranscriptFollowsSameHostRedirect verifies that a redirect staying on
// an allowlisted host is still followed and yields the transcript segments.
func TestTranscriptFollowsSameHostRedirect(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/timedtext", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/tt2", http.StatusFound)
	})
	mux.HandleFunc("/tt2", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(msXML))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	allowLoopbackTranscriptHost(t)

	got, err := NewClient().Transcript(context.Background(), srv.URL+"/timedtext")
	require.NoError(t, err)
	assert.Equal(t, wantSegments, got)
}
