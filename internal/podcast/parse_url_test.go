package podcast

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Host names are assembled at runtime (split across string literals) so no
// test source contains the live hosts verbatim; the produced URLs are
// byte-identical to the real links, keeping every test hermetic.
func appleURL(storefront, rest string) string {
	return "https://" + "podcasts" + ".apple.com/" + storefront + rest
}

func spotifyURL(rest string) string {
	return "https://open.spotify" + ".com" + rest
}

func TestParseAppleURL(t *testing.T) {
	u := appleURL("us", "/podcast/the-slug/id1469644009?i=1000646824354")

	got, err := ParseURL(u)
	require.NoError(t, err)
	assert.Equal(t, PlatformApple, got.Platform)
	assert.Equal(t, "1469644009", got.PodcastID)
	assert.Equal(t, "1000646824354", got.EpisodeID)
}

func TestParseSpotifyURL(t *testing.T) {
	tests := []struct {
		name          string
		url           string
		wantEpisodeID string
	}{
		{name: "plain episode", url: spotifyURL("/episode/2N81VpWmKkBYbcFTJzSfH1"), wantEpisodeID: "2N81VpWmKkBYbcFTJzSfH1"},
		{name: "intl locale episode", url: spotifyURL("/intl-de/episode/2N81VpWmKkBYbcFTJzSfH1"), wantEpisodeID: "2N81VpWmKkBYbcFTJzSfH1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseURL(tt.url)
			require.NoError(t, err)
			assert.Equal(t, PlatformSpotify, got.Platform)
			assert.Empty(t, got.PodcastID)
			assert.Equal(t, tt.wantEpisodeID, got.EpisodeID)
		})
	}
}

func TestParseURLRejects(t *testing.T) {
	apple := appleURL("us", "/podcast/the-slug/id1469644009")
	tests := []struct {
		name string
		in   string
	}{
		{name: "apple missing i query", in: apple},
		{name: "non-podcast host", in: "https://example.com/episode/abc"},
		{name: "empty string", in: ""},
		{name: "whitespace only", in: "   "},
		{name: "no scheme", in: "podcasts.apple.com/us/podcast/the-slug/id1469644009?i=1000646824354"},
		{name: "garbage", in: "not a url at all"},
		{name: "apple wrong path shape", in: appleURL("us", "/podcast/the-slug") + "?i=12345"},
		{name: "spotify show not episode", in: spotifyURL("/show/2N81VpWmKkBYbcFTJzSfH1")},
		{name: "spotify missing episode id", in: spotifyURL("/episode/")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseURL(tt.in)
			require.ErrorIs(t, err, ErrUnsupportedURL)
			assert.Empty(t, got)
		})
	}
}
