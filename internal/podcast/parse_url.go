// Package podcast parses Apple Podcasts and Spotify podcast episode URLs.
package podcast

import (
	"errors"
	"net/url"
	"strings"
)

// ErrUnsupportedURL is returned by ParseURL when the raw string is not a
// recognized Apple Podcasts or Spotify episode URL.
var ErrUnsupportedURL = errors.New("podcast: unsupported episode URL")

// Platform identifies which podcast service a Ref came from.
type Platform string

// Supported podcast platforms.
const (
	PlatformApple   Platform = "apple"
	PlatformSpotify Platform = "spotify"
)

// Ref is a parsed podcast episode reference.
type Ref struct {
	Platform  Platform
	PodcastID string
	EpisodeID string
}

// ParseURL parses an Apple Podcasts or Spotify episode URL. Apple episodes
// use podcasts.apple.com/<storefront>/podcast/<slug>/id<podcastId>?i=<episodeId>
// (the ?i= query is required and carries the episode ID), while Spotify
// episodes use open.spotify.com/[intl-<xx>/]episode/<base62Id>. Any other
// input yields ErrUnsupportedURL.
func ParseURL(raw string) (Ref, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Ref{}, ErrUnsupportedURL
	}
	if !strings.Contains(raw, "://") {
		return Ref{}, ErrUnsupportedURL
	}

	u, err := url.Parse(raw)
	if err != nil {
		return Ref{}, ErrUnsupportedURL
	}

	host := strings.ToLower(u.Host)
	switch {
	case host == "podcasts.apple.com":
		return parseApple(u)
	case host == "open.spotify.com":
		return parseSpotify(u)
	default:
		return Ref{}, ErrUnsupportedURL
	}
}

func parseApple(u *url.URL) (Ref, error) {
	// Path shape: /<storefront>/podcast/<slug>/id<podcastId>
	segments := splitPath(u.Path)
	if len(segments) != 4 || segments[1] != "podcast" {
		return Ref{}, ErrUnsupportedURL
	}
	slug := segments[2]
	const idPrefix = "id"
	if slug == "" || !strings.HasPrefix(segments[3], idPrefix) {
		return Ref{}, ErrUnsupportedURL
	}
	podcastID := strings.TrimPrefix(segments[3], idPrefix)
	if podcastID == "" {
		return Ref{}, ErrUnsupportedURL
	}

	// The ?i=<episodeId> query is required; it carries the episode ID.
	episodeID := u.Query().Get("i")
	if episodeID == "" {
		return Ref{}, ErrUnsupportedURL
	}

	return Ref{Platform: PlatformApple, PodcastID: podcastID, EpisodeID: episodeID}, nil
}

func parseSpotify(u *url.URL) (Ref, error) {
	// Path shape: /episode/<base62Id> or /intl-<xx>/episode/<base62Id>
	segments := splitPath(u.Path)
	if len(segments) != 2 || segments[0] != "episode" {
		if len(segments) != 3 || !strings.HasPrefix(segments[0], "intl-") || segments[1] != "episode" {
			return Ref{}, ErrUnsupportedURL
		}
		segments = segments[1:]
	}
	episodeID := segments[1]
	if episodeID == "" {
		return Ref{}, ErrUnsupportedURL
	}
	return Ref{Platform: PlatformSpotify, PodcastID: "", EpisodeID: episodeID}, nil
}

// splitPath returns the non-empty slash-separated segments of a URL path.
func splitPath(p string) []string {
	var segs []string
	for _, s := range strings.Split(strings.Trim(p, "/"), "/") {
		if s != "" {
			segs = append(segs, s)
		}
	}
	return segs
}
