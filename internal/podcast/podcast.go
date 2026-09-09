package podcast

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// Sentinel errors returned by the podcast Client.
var (
	// ErrEpisodeNotFound is returned when an episode title cannot be matched
	// to exactly one feed item (no hit in any tier, or a tier-3 ambiguity).
	ErrEpisodeNotFound = errors.New("podcast: episode not found in feed")
	// ErrNoTranscript is returned when a matched episode item carries no
	// usable podcast:transcript tag.
	ErrNoTranscript = errors.New("podcast: feed carries no usable transcript")
	// ErrEmptyTranscript is returned when a transcript URL answers 200 but
	// yields no parseable cues.
	ErrEmptyTranscript = errors.New("podcast: empty transcript")
)

const (
	// requestTimeout bounds a single feed or transcript fetch, mirroring the
	// youtube client's 30s budget.
	requestTimeout = 30 * time.Second

	// maxFetchBytes caps any feed, transcript, or metadata body read through
	// fetchBytes so a hostile server cannot balloon memory.
	maxFetchBytes = 64 << 20 // 64 MiB
)

// networkClient drives every feed and transcript fetch. Redirects are
// followed but every hop target is re-validated against the https-only (or
// loopback test server) rule, so a redirect cannot slip the client onto an
// arbitrary plain-HTTP origin.
var networkClient = &http.Client{
	Timeout: requestTimeout,
	CheckRedirect: func(req *http.Request, _ []*http.Request) error {
		return checkRedirectHTTPS(req)
	},
}

// checkRedirectHTTPS re-validates a redirect hop target: it must be https,
// or a loopback host (used by httptest test servers). Mirroring the existing
// checkHTTPS seam so tests can use httptest with no real network.
func checkRedirectHTTPS(req *http.Request) error {
	if err := checkHTTPS(req.URL); err != nil {
		return fmt.Errorf("podcast: refusing redirect to %q: %w", req.URL.Redacted(), err)
	}
	return nil
}

// Client resolves podcast episodes to their episodes and parses timed
// transcripts, using only the Go standard library.
type Client interface {
	// Episode resolves ref to an episode, fetching its show's RSS feed and
	// selecting the episode's transcript tag.
	Episode(ctx context.Context, ref Ref) (*Episode, error)
	// Transcript fetches and parses the timed caption segments behind a
	// transcript URL (the TranscriptURL of an Episode).
	Transcript(ctx context.Context, url string) ([]Segment, error)
}

// Episode is the transcript-bearing resolution of a podcast episode
// reference.
type Episode struct {
	Show          string `json:"show"`
	Title         string `json:"title"`
	Platform      string `json:"platform"`
	EpisodeID     string `json:"episode_id"`
	TranscriptURL string `json:"transcript_url"`
}

// Segment is one timed caption segment of a podcast transcript.
type Segment struct {
	StartMS    int64  `json:"start_ms"`
	DurationMS int64  `json:"duration_ms"`
	Text       string `json:"text"`
}

// client is the Client implementation.
type client struct{}

// NewClient returns a Client with sane default timeouts.
func NewClient() Client {
	return &client{}
}

// Episode resolves ref through the platform resolver, fetches and parses the
// show RSS feed, matches the episode item by title, and selects its
// transcript tag.
func (c *client) Episode(ctx context.Context, ref Ref) (*Episode, error) {
	var meta Meta
	var err error
	switch ref.Platform {
	case PlatformApple:
		meta, err = ResolveApple(ctx, ref)
	case PlatformSpotify:
		meta, err = ResolveSpotify(ctx, ref)
	default:
		return nil, fmt.Errorf("podcast: unknown platform %q", ref.Platform)
	}
	if err != nil {
		return nil, err
	}

	feed, err := fetchFeed(ctx, meta.FeedURL)
	if err != nil {
		return nil, err
	}
	idx, err := matchTitle(meta.EpisodeTitle, feedTitles(feed))
	if err != nil {
		return nil, fmt.Errorf("podcast: matching episode %q: %w", meta.EpisodeTitle, err)
	}
	tag, err := selectTranscript(feed.Items[idx].Transcripts, feed.Language)
	if err != nil {
		return nil, fmt.Errorf("podcast: selecting transcript: %w", err)
	}
	return &Episode{
		Show:          meta.ShowTitle,
		Title:         meta.EpisodeTitle,
		Platform:      string(ref.Platform),
		EpisodeID:     ref.EpisodeID,
		TranscriptURL: tag.URL,
	}, nil
}

// Transcript fetches and parses the timed caption segments behind a
// transcript URL.
func (c *client) Transcript(ctx context.Context, rawURL string) ([]Segment, error) {
	data, err := fetchBytes(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	segs, err := parseCues(data)
	if err != nil {
		return nil, fmt.Errorf("podcast: parsing transcript: %w", err)
	}
	if len(segs) == 0 {
		// A 200 with zero parseable cues must not read as a silent success.
		return nil, ErrEmptyTranscript
	}
	return segs, nil
}

// feedTitles returns the item titles of a parsed feed, in feed order.
func feedTitles(f *feed) []string {
	out := make([]string, len(f.Items))
	for i, it := range f.Items {
		out[i] = it.Title
	}
	return out
}
