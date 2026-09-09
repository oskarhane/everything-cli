package podcast

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetRedirectToPlainHTTPRejected proves the shared pipeline's
// CheckRedirect re-validates every hop: a redirect off a loopback test server
// onto a non-loopback plain-HTTP origin is rejected for metadata fetches just
// as it is for feed/transcript fetches (rss_test.go covers the feed path).
func TestGetRedirectToPlainHTTPRejected(t *testing.T) {
	t.Run("iTunes lookup", func(t *testing.T) {
		setSeams(t)
		// The loopback server itself is allowed; its plain-HTTP redirect
		// target is not.
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "http://example.com/lookup", http.StatusFound)
		}))
		t.Cleanup(srv.Close)
		itunesLookupEndpoint = srv.URL

		_, err := lookupShow(context.Background(), "1469644009")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "redirect")
	})

	t.Run("apple episode page", func(t *testing.T) {
		setSeams(t)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "http://example.com/podcast", http.StatusMovedPermanently)
		}))
		t.Cleanup(srv.Close)
		appleEpisodePage = srv.URL

		_, err := ResolveApple(context.Background(), appleRef())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "redirect")
	})
}

// TestResolveAppleFollowsRedirect proves the Apple episode page fetch still
// follows the real 301 the live page answers on first hit, landing on a
// loopback target that serves the page.
func TestResolveAppleFollowsRedirect(t *testing.T) {
	setSeams(t)
	var gotUA string
	targetSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(applePageHTML))
	}))
	t.Cleanup(targetSrv.Close)
	pageSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, targetSrv.URL+r.URL.String(), http.StatusMovedPermanently)
	}))
	t.Cleanup(pageSrv.Close)
	lookupSrv := newJSONServer(t, func(_ *http.Request) ([]byte, int) {
		return []byte(itunesResultsFixture(itunesShow{CollectionName: "The Show", FeedURL: "https://feed.example/rss"})), http.StatusOK
	})
	appleEpisodePage = pageSrv.URL
	itunesLookupEndpoint = lookupSrv

	got, err := ResolveApple(context.Background(), appleRef())
	require.NoError(t, err)
	assert.Equal(t, "Episode 100 & Season Finale", got.EpisodeTitle)
	assert.Contains(t, gotUA, "Mozilla/5.0", "browser User-Agent survives the redirect")
}

// TestGetUserAgentPerCaller asserts only the Apple episode page fetch carries
// a browser User-Agent; every other caller leaves the header unset (Go's
// default UA).
func TestGetUserAgentPerCaller(t *testing.T) {
	setSeams(t)
	const goDefaultUA = "Go-http-client/1.1"

	t.Run("iTunes lookup sends no UA", func(t *testing.T) {
		var gotUA string
		itunesLookupEndpoint = newJSONServer(t, func(r *http.Request) ([]byte, int) {
			gotUA = r.Header.Get("User-Agent")
			return []byte(itunesResultsFixture(itunesShow{CollectionName: "S", FeedURL: "https://feed.example/rss"})), http.StatusOK
		})
		_, err := lookupShow(context.Background(), "1")
		require.NoError(t, err)
		assert.Equal(t, goDefaultUA, gotUA)
	})

	t.Run("iTunes search sends no UA", func(t *testing.T) {
		var gotUA string
		itunesSearchEndpoint = newJSONServer(t, func(r *http.Request) ([]byte, int) {
			gotUA = r.Header.Get("User-Agent")
			return []byte(itunesResultsFixture(itunesShow{CollectionName: "S", FeedURL: "https://feed.example/rss"})), http.StatusOK
		})
		_, err := searchShow(context.Background(), "S")
		require.NoError(t, err)
		assert.Equal(t, goDefaultUA, gotUA)
	})

	t.Run("spotify embed page sends no UA", func(t *testing.T) {
		var gotUA string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotUA = r.Header.Get("User-Agent")
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(spotifyEmbedHTML))
		}))
		t.Cleanup(srv.Close)
		spotifyEmbedPage = srv.URL
		itunesSearchEndpoint = newJSONServer(t, func(_ *http.Request) ([]byte, int) {
			return []byte(itunesResultsFixture(itunesShow{CollectionName: "The Show", FeedURL: "https://feed.example/rss"})), http.StatusOK
		})
		_, err := ResolveSpotify(context.Background(), spotifyRef())
		require.NoError(t, err)
		assert.Equal(t, goDefaultUA, gotUA)
	})

	t.Run("feed fetch sends no UA", func(t *testing.T) {
		var gotUA string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotUA = r.Header.Get("User-Agent")
			_, _ = w.Write([]byte(pc20Feed))
		}))
		t.Cleanup(srv.Close)
		_, err := fetchFeed(context.Background(), srv.URL+"/feed.xml")
		require.NoError(t, err)
		assert.Equal(t, goDefaultUA, gotUA)
	})

	t.Run("transcript fetch sends no UA", func(t *testing.T) {
		var gotUA string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotUA = r.Header.Get("User-Agent")
			_, _ = w.Write([]byte(vttFixture))
		}))
		t.Cleanup(srv.Close)
		_, err := NewClient().Transcript(context.Background(), srv.URL+"/t.vtt")
		require.NoError(t, err)
		assert.Equal(t, goDefaultUA, gotUA)
	})
}
