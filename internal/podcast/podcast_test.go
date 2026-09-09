package podcast

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTranscriptServer serves body over httptest for fetch-based tests.
func newTranscriptServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// feedWithItem builds a minimal PC20-style RSS feed with a single item whose
// title and transcript tag URL are parameterized.
func feedWithItem(t *testing.T, title, transcriptURL, lang string) string {
	t.Helper()
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:podcast="https://podcastindex.org/namespace/1.0">
<channel>
<title>Show</title>
<language>%s</language>
<item>
<title>%s</title>
<podcast:transcript url="%s" type="text/vtt" rel="captions" language="en"/>
</item>
</channel>
</rss>`, lang, html.EscapeString(title), transcriptURL)
}

func TestEpisodeAppleEndToEnd(t *testing.T) {
	setSeams(t)
	transcriptSrv := newTranscriptServer(t, vttFixture)
	feedSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(feedWithItem(t, "Episode 100 & Season Finale", transcriptSrv.URL, "en")))
	}))
	t.Cleanup(feedSrv.Close)

	pageSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(applePageHTML)) // og:title "Episode 100 & Season Finale"
	}))
	t.Cleanup(pageSrv.Close)
	lookupSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(itunesResultsFixture(itunesShow{CollectionName: "The Show", FeedURL: feedSrv.URL})))
	}))
	t.Cleanup(lookupSrv.Close)
	appleEpisodePage = pageSrv.URL
	itunesLookupEndpoint = lookupSrv.URL

	ep, err := NewClient().Episode(context.Background(), appleRef())
	require.NoError(t, err)
	assert.Equal(t, "The Show", ep.Show)
	assert.Equal(t, "Episode 100 & Season Finale", ep.Title)
	assert.Equal(t, "apple", ep.Platform)
	assert.Equal(t, "1000646824354", ep.EpisodeID)
	assert.Equal(t, transcriptSrv.URL, ep.TranscriptURL)

	segs, err := NewClient().Transcript(context.Background(), ep.TranscriptURL)
	require.NoError(t, err)
	assert.Equal(t, wantCues, segs)
}

func TestEpisodeSpotifyEndToEnd(t *testing.T) {
	setSeams(t)
	transcriptSrv := newTranscriptServer(t, srtFixture)
	feedSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(feedWithItem(t, "Great Episode", transcriptSrv.URL, "en")))
	}))
	t.Cleanup(feedSrv.Close)

	embedSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><head><script id="__NEXT_DATA__">` +
			`{"props":{"pageProps":{"state":{"data":{"entity":{"subtitle":"Show Title","title":"Great Episode"}}}}}}` +
			`</script></head></html>`))
	}))
	t.Cleanup(embedSrv.Close)
	searchSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(itunesResultsFixture(itunesShow{CollectionName: "Show Title", FeedURL: feedSrv.URL})))
	}))
	t.Cleanup(searchSrv.Close)
	spotifyEmbedPage = embedSrv.URL
	itunesSearchEndpoint = searchSrv.URL

	ref := Ref{Platform: PlatformSpotify, PodcastID: "", EpisodeID: "4aBc9D"}
	ep, err := NewClient().Episode(context.Background(), ref)
	require.NoError(t, err)
	assert.Equal(t, "Show Title", ep.Show)
	assert.Equal(t, "Great Episode", ep.Title)
	assert.Equal(t, "spotify", ep.Platform)
	assert.Equal(t, "4aBc9D", ep.EpisodeID)
	assert.Equal(t, transcriptSrv.URL, ep.TranscriptURL)

	segs, err := NewClient().Transcript(context.Background(), ep.TranscriptURL)
	require.NoError(t, err)
	assert.Equal(t, wantCues, segs)
}

func TestEpisodeEpisodeNotFound(t *testing.T) {
	setSeams(t)
	feedSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(feedWithItem(t, "Some Unrelated Episode", "https://transcripts.example/x.vtt", "en")))
	}))
	t.Cleanup(feedSrv.Close)
	pageSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(applePageHTML)) // og:title "Episode 100 & Season Finale"
	}))
	t.Cleanup(pageSrv.Close)
	lookupSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(itunesResultsFixture(itunesShow{CollectionName: "The Show", FeedURL: feedSrv.URL})))
	}))
	t.Cleanup(lookupSrv.Close)
	appleEpisodePage = pageSrv.URL
	itunesLookupEndpoint = lookupSrv.URL

	_, err := NewClient().Episode(context.Background(), appleRef())
	require.Error(t, err)
	require.ErrorIs(t, err, ErrEpisodeNotFound)
}

func TestTranscript(t *testing.T) {
	t.Run("vtt body parses", func(t *testing.T) {
		srv := newTranscriptServer(t, vttFixture)
		segs, err := NewClient().Transcript(context.Background(), srv.URL+"/t.vtt")
		require.NoError(t, err)
		assert.Equal(t, wantCues, segs)
	})

	t.Run("srt body parses", func(t *testing.T) {
		srv := newTranscriptServer(t, srtFixture)
		segs, err := NewClient().Transcript(context.Background(), srv.URL+"/t.srt")
		require.NoError(t, err)
		assert.Equal(t, wantCues, segs)
	})

	t.Run("empty 200 is ErrEmptyTranscript", func(t *testing.T) {
		srv := newTranscriptServer(t, "")
		_, err := NewClient().Transcript(context.Background(), srv.URL+"/t.vtt")
		require.ErrorIs(t, err, ErrEmptyTranscript)
	})

	t.Run("200 with no cues is ErrEmptyTranscript", func(t *testing.T) {
		srv := newTranscriptServer(t, "WEBVTT\n\nKind: captions\n")
		_, err := NewClient().Transcript(context.Background(), srv.URL+"/t.vtt")
		require.ErrorIs(t, err, ErrEmptyTranscript)
	})

	t.Run("non-200 names the status", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "nope", http.StatusNotFound)
		}))
		t.Cleanup(srv.Close)
		_, err := NewClient().Transcript(context.Background(), srv.URL+"/t.vtt")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "404")
	})
}

func TestTranscriptRejectsNonHTTPSAndRedirect(t *testing.T) {
	t.Run("non-https initial URL rejected", func(t *testing.T) {
		_, err := NewClient().Transcript(context.Background(), "http://example.com/transcript")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "https")
	})

	t.Run("redirect off loopback rejected", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "http://example.com/transcript", http.StatusFound)
		}))
		t.Cleanup(srv.Close)
		_, err := NewClient().Transcript(context.Background(), srv.URL+"/t.vtt")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "redirect")
	})
}
