// Package youtube implements an unofficial YouTube InnerTube client for
// video metadata and timed transcripts, using only the Go standard library.
// The endpoints and payloads are the undocumented ones used by the official
// Android app; this is not a Google API client.
package youtube

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// Sentinel errors returned by ParseVideoID, SelectTrack, and the Client.
var (
	// ErrBadVideoID is returned when a string cannot be recognized as a
	// YouTube video reference.
	ErrBadVideoID = errors.New("invalid YouTube video ID")
	// ErrNoCaptions is returned when a video exposes no caption tracks.
	ErrNoCaptions = errors.New("no caption tracks available")
	// ErrEmptyTranscript is returned when a transcript endpoint yields no
	// parseable caption content (e.g. an HTTP 200 with an empty body).
	ErrEmptyTranscript = errors.New("empty transcript")
	// ErrUnplayable is returned when YouTube reports that a video cannot be
	// played; the wrapped message carries the playability reason.
	ErrUnplayable = errors.New("video is not playable")
)

const (
	// playerAPIKey is the well-known InnerTube Android API key shipped in
	// the official mobile app. Verified live on 2026-09-01 that the player
	// endpoint answers this key with playabilityStatus OK plus videoDetails
	// and captionTracks.
	playerAPIKey = "AIzaSyAO_FJ2SlqU8Q4STEHLGCilw_Y9_11qcW8"

	// playerClientVersion is the Android app release the API key and client
	// payload are tied to; the endpoints reject newer/unknown versions.
	playerClientVersion = "20.10.38"

	// androidSDKVersion is the Android SDK level the app client advertises.
	androidSDKVersion = 30

	// mobileUserAgent mimics the official Android app so InnerTube answers
	// with the ANDROID client payload, which includes raw caption track
	// URLs (the web client does not expose them).
	mobileUserAgent = "com.google.android.youtube/20.10.38 (Linux; U; Android 11; en_US; 20.10.38) gzip"

	// maxResponseBytes caps any single response body. A 64 MiB budget
	// comfortably covers even word-timed transcripts of multi-hour videos
	// while bounding memory against a hostile response.
	maxResponseBytes = 64 << 20

	// requestTimeout bounds a single endpoints call; transcript and player
	// fetches are small and should complete in well under this.
	requestTimeout = 30 * time.Second
)

// playerEndpoint is the InnerTube player endpoint with the API key baked
// in. It is a package-level seam so tests can point it at an
// httptest.Server's URL.
var playerEndpoint = "https://www.youtube.com/youtubei/v1/player?key=" + playerAPIKey

// Client fetches video metadata and transcripts from YouTube's unofficial
// InnerTube endpoints.
type Client interface {
	// Player fetches player metadata for a video.
	Player(ctx context.Context, videoID string) (*Player, error)
	// Transcript fetches the timed caption segments behind a caption track
	// URL (the BaseURL of a Track).
	Transcript(ctx context.Context, trackURL string) ([]Segment, error)
}

// Player is the metadata YouTube returns for a single video, including the
// caption tracks it offers.
type Player struct {
	VideoID       string  `json:"video_id"`
	Title         string  `json:"title"`
	Author        string  `json:"author"`
	ChannelID     string  `json:"channel_id"`
	LengthSeconds int64   `json:"length_seconds"`
	ViewCount     int64   `json:"view_count"`
	PublishDate   string  `json:"publish_date"`
	UploadDate    string  `json:"upload_date"`
	Category      string  `json:"category"`
	Description   string  `json:"description"`
	Tracks        []Track `json:"tracks"`
}

// Track is one caption track offered for a video.
type Track struct {
	Lang      string `json:"lang"`
	Generated bool   `json:"generated"`
	BaseURL   string `json:"base_url"`
}

// Segment is one timed caption segment of a transcript.
type Segment struct {
	StartMS    int64  `json:"start_ms"`
	DurationMS int64  `json:"duration_ms"`
	Text       string `json:"text"`
}

// HTTPClient is the Client implementation backed by YouTube's InnerTube
// endpoints.
type HTTPClient struct {
	httpClient *http.Client
	userAgent  string
}

// NewClient returns a Client with sane default timeouts.
func NewClient() *HTTPClient {
	return &HTTPClient{
		httpClient: &http.Client{Timeout: requestTimeout},
		userAgent:  mobileUserAgent,
	}
}

// Player fetches player metadata for a video.
func (c *HTTPClient) Player(ctx context.Context, videoID string) (*Player, error) {
	if _, err := ParseVideoID(videoID); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(playerRequest{
		Context: playerContext{Client: playerClient{
			ClientName:        "ANDROID",
			ClientVersion:     playerClientVersion,
			AndroidSDKVersion: androidSDKVersion,
			HL:                "en",
		}},
		VideoID: videoID,
	})
	if err != nil {
		return nil, fmt.Errorf("encoding player request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, playerEndpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("building player request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("requesting player endpoint: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("player endpoint returned status %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("reading player response: %w", err)
	}
	if int64(len(data)) > maxResponseBytes {
		return nil, fmt.Errorf("player response exceeds %d bytes", maxResponseBytes)
	}

	var pr playerResponse
	if err := json.Unmarshal(data, &pr); err != nil {
		return nil, fmt.Errorf("parsing player response: %w", err)
	}
	if pr.PlayabilityStatus.Status != "OK" {
		reason := pr.PlayabilityStatus.Reason
		if reason == "" {
			reason = pr.PlayabilityStatus.Status
		}
		if reason == "" {
			reason = "unknown reason"
		}
		return nil, fmt.Errorf("%w: %s", ErrUnplayable, reason)
	}

	p := &Player{
		VideoID:       pr.VideoDetails.VideoID,
		Title:         pr.VideoDetails.Title,
		Author:        pr.VideoDetails.Author,
		ChannelID:     pr.VideoDetails.ChannelID,
		LengthSeconds: parseCount(pr.VideoDetails.LengthSeconds),
		ViewCount:     parseCount(pr.VideoDetails.ViewCount),
		PublishDate:   pr.Microformat.PlayerMicroformatRenderer.PublishDate,
		UploadDate:    pr.Microformat.PlayerMicroformatRenderer.UploadDate,
		Category:      pr.Microformat.PlayerMicroformatRenderer.Category,
		Description:   pr.VideoDetails.ShortDescription,
	}
	for _, t := range pr.Captions.PlayerCaptionsTracklistRenderer.CaptionTracks {
		p.Tracks = append(p.Tracks, Track{
			Lang:      t.LanguageCode,
			Generated: t.Kind == "asr" || t.IsGenerated,
			BaseURL:   t.BaseURL,
		})
	}
	return p, nil
}

// Transcript fetches and parses the timed caption segments behind a caption
// track URL.
func (c *HTTPClient) Transcript(ctx context.Context, trackURL string) ([]Segment, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, trackURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building transcript request: %w", err)
	}
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("requesting transcript endpoint: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("transcript endpoint returned status %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("reading transcript response: %w", err)
	}
	if int64(len(data)) > maxResponseBytes {
		return nil, fmt.Errorf("transcript response exceeds %d bytes", maxResponseBytes)
	}

	segs, err := parseTranscriptBody(data)
	if err != nil {
		return nil, fmt.Errorf("parsing transcript: %w", err)
	}
	if len(segs) == 0 {
		// YouTube's PoToken gate answers the timedtext URL with HTTP 200 and
		// zero bytes; never report that as a silent empty success.
		return nil, ErrEmptyTranscript
	}
	return segs, nil
}

// playerRequest mirrors the JSON the official Android app posts to the
// player endpoint.
type playerRequest struct {
	Context playerContext `json:"context"`
	VideoID string        `json:"videoId"`
}

type playerContext struct {
	Client playerClient `json:"client"`
}

type playerClient struct {
	ClientName        string `json:"clientName"`
	ClientVersion     string `json:"clientVersion"`
	AndroidSDKVersion int    `json:"androidSdkVersion"`
	HL                string `json:"hl"`
}

// playerResponse mirrors the InnerTube player response fields mapped into
// Player.
type playerResponse struct {
	PlayabilityStatus struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
	} `json:"playabilityStatus"`
	VideoDetails struct {
		VideoID          string `json:"videoId"`
		Title            string `json:"title"`
		Author           string `json:"author"`
		ChannelID        string `json:"channelId"`
		LengthSeconds    string `json:"lengthSeconds"`
		ViewCount        string `json:"viewCount"`
		ShortDescription string `json:"shortDescription"`
	} `json:"videoDetails"`
	Microformat struct {
		PlayerMicroformatRenderer struct {
			PublishDate string `json:"publishDate"`
			UploadDate  string `json:"uploadDate"`
			Category    string `json:"category"`
		} `json:"playerMicroformatRenderer"`
	} `json:"microformat"`
	Captions struct {
		PlayerCaptionsTracklistRenderer struct {
			CaptionTracks []struct {
				LanguageCode string `json:"languageCode"`
				Kind         string `json:"kind"`
				BaseURL      string `json:"baseUrl"`
				IsGenerated  bool   `json:"is_generated"`
			} `json:"captionTracks"`
		} `json:"playerCaptionsTracklistRenderer"`
	} `json:"captions"`
}

// parseCount converts a YouTube numeric string field ("215", "1234567")
// into an int64, defaulting to 0 when the field is absent or malformed.
func parseCount(s string) int64 {
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return v
}
