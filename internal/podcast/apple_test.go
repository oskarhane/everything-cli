package podcast

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// applePageHTML is a minimal Apple episode page carrying an og:title whose
// value includes an HTML entity (&amp;) so tests prove unescaping.
const applePageHTML = `<!DOCTYPE html><html><head>
<meta property="og:title" content="Episode 100 &amp; Season Finale">
</head><body></body></html>`

func TestExtractOGTitle(t *testing.T) {
	tests := []struct {
		name string
		page string
		want string
	}{
		{name: "double-quoted with entity", page: `<meta property="og:title" content="A &amp; B">`, want: "A & B"},
		{name: "single-quoted", page: `<meta property='og:title' content='Loose &quot;Goose&quot;'>`, want: `Loose "Goose"`},
		{name: "content before property", page: `<meta content="Reversed" property="og:title">`, want: "Reversed"},
		{name: "entity number", page: `<meta property="og:title" content="Coin &euro;">`, want: "Coin €"},
		{name: "not og:title", page: `<meta property="og:type" content="website">`},
		{name: "no meta", page: `<html><body>nope</body></html>`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, extractOGTitle(tt.page))
		})
	}
}

// newAppleServers wires up a fake Apple episode page (optionally a real one)
// and a fake iTunes lookup, swapping the seams and restoring them on cleanup.
func newAppleServers(t *testing.T, pageStatus int) (*httptest.Server, *httptest.Server, *string) {
	t.Helper()
	var userAgent string
	pageSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userAgent = r.Header.Get("User-Agent")
		if pageStatus != http.StatusOK {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(applePageHTML))
	}))
	t.Cleanup(pageSrv.Close)

	lookupSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("id") == "" {
			http.Error(w, "missing id", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(itunesResultsFixture(itunesShow{CollectionName: "The Show", FeedURL: "https://feed.example/rss"})))
	}))
	t.Cleanup(lookupSrv.Close)

	setSeams(t)
	appleEpisodePage = pageSrv.URL
	itunesLookupEndpoint = lookupSrv.URL
	return pageSrv, lookupSrv, &userAgent
}

func appleRef() Ref {
	return Ref{Platform: PlatformApple, PodcastID: "1469644009", EpisodeID: "1000646824354"}
}

func TestResolveAppleSuccess(t *testing.T) {
	_, _, ua := newAppleServers(t, http.StatusOK)

	got, err := ResolveApple(context.Background(), appleRef())
	require.NoError(t, err)
	assert.Equal(t, "The Show", got.ShowTitle)
	assert.Equal(t, "Episode 100 & Season Finale", got.EpisodeTitle)
	assert.Equal(t, "https://feed.example/rss", got.FeedURL)
	assert.Contains(t, *ua, "Mozilla/5.0", "Apple page must be fetched with a browser User-Agent")
}

func TestResolveAppleLookupMiss(t *testing.T) {
	setSeams(t)
	pageSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(applePageHTML))
	}))
	t.Cleanup(pageSrv.Close)
	lookupSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(itunesResultsFixture()))
	}))
	t.Cleanup(lookupSrv.Close)
	appleEpisodePage = pageSrv.URL
	itunesLookupEndpoint = lookupSrv.URL

	_, err := ResolveApple(context.Background(), appleRef())
	require.ErrorIs(t, err, ErrShowNotFound)
}

func TestResolveAppleWrongPlatform(t *testing.T) {
	setSeams(t)
	_, err := ResolveApple(context.Background(), Ref{Platform: PlatformSpotify, EpisodeID: "x"})
	require.Error(t, err)
}
