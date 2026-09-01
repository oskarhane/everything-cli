package youtube

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// okPlayerBody is a realistic InnerTube player response: playabilityStatus
// OK, videoDetails, microformat dates, and three caption tracks (human en,
// asr en, and an is_generated es track). Track base URLs point at an inert
// host; the player test never fetches them.
const okPlayerBody = `{
  "playabilityStatus": {"status": "OK", "reason": ""},
  "videoDetails": {
    "videoId": "dQw4w9WgXcQ",
    "title": "Test Video",
    "author": "Test Author",
    "channelId": "UC1234567890abcdef",
    "lengthSeconds": "215",
    "viewCount": "1234567",
    "shortDescription": "A description."
  },
  "microformat": {
    "playerMicroformatRenderer": {
      "publishDate": "2024-01-15T00:00:00Z",
      "uploadDate": "2024-01-15T00:00:00Z",
      "category": "Music"
    }
  },
  "captions": {
    "playerCaptionsTracklistRenderer": {
      "captionTracks": [
        {"languageCode": "en", "kind": "", "baseUrl": "https://example.com/tt?lang=en"},
        {"languageCode": "en", "kind": "asr", "baseUrl": "https://example.com/tt?lang=en&kind=asr"},
        {"languageCode": "es", "kind": "", "is_generated": true, "baseUrl": "https://example.com/tt?lang=es"}
      ]
    }
  }
}`

// usePlayerEndpoint redirects the package-level player endpoint seam at an
// httptest.Server for the duration of the test, keeping the ?key= query
// parameter the production endpoint carries.
func usePlayerEndpoint(t *testing.T, srv *httptest.Server) {
	t.Helper()
	orig := playerEndpoint
	playerEndpoint = srv.URL + "/youtubei/v1/player?key=" + playerAPIKey
	t.Cleanup(func() { playerEndpoint = orig })
}

func TestPlayer(t *testing.T) {
	ctx := context.Background()

	t.Run("posts android payload and maps metadata", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "/youtubei/v1/player", r.URL.Path)
			assert.Equal(t, playerAPIKey, r.URL.Query().Get("key"))
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			assert.Contains(t, r.UserAgent(), "com.google.android.youtube")

			var req playerRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "dQw4w9WgXcQ", req.VideoID)
			assert.Equal(t, "ANDROID", req.Context.Client.ClientName)
			assert.Equal(t, playerClientVersion, req.Context.Client.ClientVersion)
			assert.Equal(t, androidSDKVersion, req.Context.Client.AndroidSDKVersion)
			assert.Equal(t, "en", req.Context.Client.HL)
			_, _ = w.Write([]byte(okPlayerBody))
		}))
		usePlayerEndpoint(t, srv)

		got, err := NewClient().Player(ctx, "dQw4w9WgXcQ")
		require.NoError(t, err)
		assert.Equal(t, "dQw4w9WgXcQ", got.VideoID)
		assert.Equal(t, "Test Video", got.Title)
		assert.Equal(t, "Test Author", got.Author)
		assert.Equal(t, "UC1234567890abcdef", got.ChannelID)
		assert.Equal(t, int64(215), got.LengthSeconds)
		assert.Equal(t, int64(1234567), got.ViewCount)
		assert.Equal(t, "2024-01-15T00:00:00Z", got.PublishDate)
		assert.Equal(t, "2024-01-15T00:00:00Z", got.UploadDate)
		assert.Equal(t, "Music", got.Category)
		assert.Equal(t, "A description.", got.Description)

		require.Len(t, got.Tracks, 3)
		assert.Equal(t, "en", got.Tracks[0].Lang)
		assert.False(t, got.Tracks[0].Generated, "kind empty is a human track")
		assert.Equal(t, "en", got.Tracks[1].Lang)
		assert.True(t, got.Tracks[1].Generated, "kind asr is a generated track")
		assert.Equal(t, "es", got.Tracks[2].Lang)
		assert.True(t, got.Tracks[2].Generated, "is_generated flag marks the track generated")
	})

	t.Run("unplayable status wraps the reason", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{
				"playabilityStatus": {
					"status": "LOGIN_REQUIRED",
					"reason": "Sign in to confirm you're not a bot"
				},
				"videoDetails": {"videoId": "dQw4w9WgXcQ"}
			}`))
		}))
		usePlayerEndpoint(t, srv)

		_, err := NewClient().Player(ctx, "dQw4w9WgXcQ")
		require.ErrorIs(t, err, ErrUnplayable)
		assert.Contains(t, err.Error(), "Sign in to confirm you're not a bot")
	})

	t.Run("non-200 names the status code", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "nope", http.StatusForbidden)
		}))
		usePlayerEndpoint(t, srv)

		_, err := NewClient().Player(ctx, "dQw4w9WgXcQ")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "403")
	})

	t.Run("invalid video ID is rejected before any request", func(t *testing.T) {
		_, err := NewClient().Player(ctx, "not-a-valid-id")
		require.ErrorIs(t, err, ErrBadVideoID)
	})
}
