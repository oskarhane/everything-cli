package podcast

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setSeams restores every package-level endpoint/client seam to its
// production value on cleanup, so each test is independent.
func setSeams(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		itunesSearchEndpoint = "https://itunes.apple.com/search"
		itunesLookupEndpoint = "https://itunes.apple.com/lookup"
		appleEpisodePage = "https://podcasts.apple.com"
		spotifyEmbedPage = "https://open.spotify.com/embed"
	})
}

// itunesResultsFixture marshals an iTunes Search API response envelope shaped
// like the real endpoint (resultCount + results[] with collectionName/feedUrl).
func itunesResultsFixture(results ...itunesShow) string {
	b, err := json.Marshal(map[string]any{
		"resultCount": len(results),
		"results":     results,
	})
	if err != nil {
		panic(err)
	}
	return string(b)
}

func TestLookupShowSuccess(t *testing.T) {
	setSeams(t)
	var gotQuery string
	itunesLookupEndpoint = newJSONServer(t, func(r *http.Request) ([]byte, int) {
		gotQuery = r.URL.RawQuery
		return []byte(itunesResultsFixture(
			itunesShow{CollectionName: "The Show", FeedURL: "https://feed.example/rss"},
		)), http.StatusOK
	})

	got, err := lookupShow(context.Background(), "1469644009")
	require.NoError(t, err)
	assert.Equal(t, "The Show", got.CollectionName)
	assert.Equal(t, "https://feed.example/rss", got.FeedURL)
	assert.Contains(t, gotQuery, "id=1469644009")
}

func TestLookupShowMiss(t *testing.T) {
	setSeams(t)
	itunesLookupEndpoint = newJSONServer(t, func(r *http.Request) ([]byte, int) {
		return []byte(itunesResultsFixture()), http.StatusOK
	})

	_, err := lookupShow(context.Background(), "9999999999")
	require.ErrorIs(t, err, ErrShowNotFound)
}

func TestSearchShowPrefersExactMatch(t *testing.T) {
	setSeams(t)
	itunesSearchEndpoint = newJSONServer(t, func(r *http.Request) ([]byte, int) {
		return []byte(itunesResultsFixture(
			itunesShow{CollectionName: "Similar Name", FeedURL: "https://feed.example/similar"},
			itunesShow{CollectionName: "Exact Show Name", FeedURL: "https://feed.example/exact"},
		)), http.StatusOK
	})

	got, err := searchShow(context.Background(), "Exact Show Name")
	require.NoError(t, err)
	assert.Equal(t, "Exact Show Name", got.CollectionName)
	assert.Equal(t, "https://feed.example/exact", got.FeedURL)
}

func TestSearchShowFallbackToFirstResult(t *testing.T) {
	setSeams(t)
	itunesSearchEndpoint = newJSONServer(t, func(r *http.Request) ([]byte, int) {
		return []byte(itunesResultsFixture(
			itunesShow{CollectionName: "First Result", FeedURL: "https://feed.example/first"},
			itunesShow{CollectionName: "No Exact", FeedURL: "https://feed.example/second"},
		)), http.StatusOK
	})

	got, err := searchShow(context.Background(), "Does Not Match")
	require.NoError(t, err)
	assert.Equal(t, "First Result", got.CollectionName)
	assert.Equal(t, "https://feed.example/first", got.FeedURL)
}

func TestSearchShowMiss(t *testing.T) {
	setSeams(t)
	itunesSearchEndpoint = newJSONServer(t, func(r *http.Request) ([]byte, int) {
		return []byte(itunesResultsFixture()), http.StatusOK
	})

	_, err := searchShow(context.Background(), "No Such Show")
	require.ErrorIs(t, err, ErrShowNotFound)
}

func TestSearchShowEncodesUTF8Term(t *testing.T) {
	setSeams(t)
	var gotQuery string
	itunesSearchEndpoint = newJSONServer(t, func(r *http.Request) ([]byte, int) {
		gotQuery = r.URL.RawQuery
		return []byte(itunesResultsFixture(
			itunesShow{CollectionName: "Gïde & Garn", FeedURL: "https://feed.example/rss"},
		)), http.StatusOK
	})

	_, err := searchShow(context.Background(), "Gïde & Garn")
	require.NoError(t, err)
	assert.Contains(t, gotQuery, "entity=podcast")
	assert.Contains(t, gotQuery, "G%C3%AFde+%26+Garn")
}

// newJSONServer spins up an httptest server whose handler is supplied per
// request and which always writes the given content-type.
func newJSONServer(t *testing.T, fn func(r *http.Request) ([]byte, int)) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, status := fn(r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}
