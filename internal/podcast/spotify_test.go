package podcast

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// spotifyEmbedHTML mirrors a real Spotify embed page: server-rendered HTML
// carrying a __NEXT_DATA__ script whose JSON holds the show title
// (entity.subtitle) and episode title (entity.title).
const spotifyEmbedHTML = `<!DOCTYPE html><html><head>
<script id="__NEXT_DATA__" type="application/json">{"props":{"pageProps":{"state":{"data":{"entity":{"subtitle":"The Show","title":"Episode Title"}}}}}}</script>
</head><body></body></html>`

func spotifyRef() Ref {
	return Ref{Platform: PlatformSpotify, PodcastID: "", EpisodeID: "2N81VpWmKkBYbcFTJzSfH1"}
}

func TestResolveSpotifySuccess(t *testing.T) {
	setSeams(t)
	var gotTerm string
	spotifyEmbedPage = newHTMLServer(t, func() string { return spotifyEmbedHTML })
	itunesSearchEndpoint = newJSONServer(t, func(r *http.Request) ([]byte, int) {
		gotTerm = r.URL.Query().Get("term")
		return []byte(itunesResultsFixture(
			itunesShow{CollectionName: "The Show", FeedURL: "https://feed.example/rss"},
		)), http.StatusOK
	})

	got, err := ResolveSpotify(context.Background(), spotifyRef())
	require.NoError(t, err)
	assert.Equal(t, "The Show", got.ShowTitle)
	assert.Equal(t, "Episode Title", got.EpisodeTitle)
	assert.Equal(t, "https://feed.example/rss", got.FeedURL)
	assert.Equal(t, "The Show", gotTerm)
}

func TestResolveSpotifyDeadEpisode(t *testing.T) {
	setSeams(t)
	spotifyEmbedPage = newStatusServer(t, http.StatusNotFound)

	_, err := ResolveSpotify(context.Background(), spotifyRef())
	require.ErrorIs(t, err, ErrShowNotFound)
}

func TestResolveSpotifyEmbedWithoutNextData(t *testing.T) {
	setSeams(t)
	spotifyEmbedPage = newHTMLServer(t, func() string { return "<html><body>degraded page</body></html>" })

	_, err := ResolveSpotify(context.Background(), spotifyRef())
	require.ErrorIs(t, err, ErrShowNotFound)
}

func TestResolveSpotifySearchMiss(t *testing.T) {
	setSeams(t)
	spotifyEmbedPage = newHTMLServer(t, func() string { return spotifyEmbedHTML })
	itunesSearchEndpoint = newJSONServer(t, func(r *http.Request) ([]byte, int) {
		return []byte(itunesResultsFixture()), http.StatusOK
	})

	_, err := ResolveSpotify(context.Background(), spotifyRef())
	require.ErrorIs(t, err, ErrShowNotFound)
	assert.Contains(t, err.Error(), "The Show", "miss must wrap the search term for debugging")
}

// newHTMLServer serves the given HTML body at any path.
func newHTMLServer(t *testing.T, render func() string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(render()))
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// newStatusServer serves the given HTTP status at any path.
func newStatusServer(t *testing.T, status int) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, http.StatusText(status), status)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}
