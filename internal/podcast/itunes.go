package podcast

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// ErrShowNotFound is returned when a podcast show cannot be resolved: the
// iTunes lookup/search found no matching show, or a dead/stale episode page
// was unreachable. It is a sentinel so callers can test with errors.Is.
var ErrShowNotFound = errors.New("podcast: show not found")

// Meta is the resolved metadata for a podcast episode: the show and episode
// titles plus the show's RSS feed URL.
type Meta struct {
	ShowTitle    string
	EpisodeTitle string
	FeedURL      string
}

// itunesSearchEndpoint and itunesLookupEndpoint are the iTunes Search API
// base URLs. They are package-level seams: production always points at the
// live https origins, while tests swap them for httptest servers.
var (
	itunesSearchEndpoint = "https://itunes.apple.com/search"
	itunesLookupEndpoint = "https://itunes.apple.com/lookup"
)

// httpClient drives every outbound metadata/reference fetch. The timeout
// bounds each call so a hung endpoint cannot stall resolution indefinitely;
// redirects are followed (the http.Client default), which the Apple episode
// page needs — it answers a 301 on first hit.
var httpClient = &http.Client{Timeout: 30 * time.Second}

// maxBodyBytes caps how much HTML/JSON a remote endpoint may send before we
// stop reading, so a misbehaving server cannot balloon memory.
const maxBodyBytes = 64 << 20 // 64 MiB

// itunesShow is the subset of an iTunes Search API result row the resolvers
// need.
type itunesShow struct {
	CollectionName string `json:"collectionName"`
	FeedURL        string `json:"feedUrl"`
}

// itunesResponse is the iTunes Search API envelope; both the lookup and the
// search endpoints share it.
type itunesResponse struct {
	ResultCount int          `json:"resultCount"`
	Results     []itunesShow `json:"results"`
}

// lookupShow resolves a podcast show by its iTunes podcast ID via the lookup
// endpoint. A resultCount of 0 is a miss -> ErrShowNotFound.
func lookupShow(ctx context.Context, podcastID string) (itunesShow, error) {
	q := url.Values{}
	q.Set("id", podcastID)
	body, err := httpGet(ctx, itunesLookupEndpoint+"?"+q.Encode(), "")
	if err != nil {
		return itunesShow{}, err
	}
	var resp itunesResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return itunesShow{}, fmt.Errorf("podcast: decoding iTunes lookup response: %w", err)
	}
	if resp.ResultCount == 0 || len(resp.Results) == 0 {
		return itunesShow{}, ErrShowNotFound
	}
	return resp.Results[0], nil
}

// searchShow finds a podcast show by title via the search endpoint
// (entity=podcast). It prefers the result whose collectionName exactly equals
// term, and falls back to the first result. A resultCount of 0 is a miss ->
// ErrShowNotFound.
func searchShow(ctx context.Context, term string) (itunesShow, error) {
	q := url.Values{}
	q.Set("entity", "podcast")
	q.Set("term", term)
	body, err := httpGet(ctx, itunesSearchEndpoint+"?"+q.Encode(), "")
	if err != nil {
		return itunesShow{}, err
	}
	var resp itunesResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return itunesShow{}, fmt.Errorf("podcast: decoding iTunes search response: %w", err)
	}
	if resp.ResultCount == 0 || len(resp.Results) == 0 {
		return itunesShow{}, ErrShowNotFound
	}
	for _, r := range resp.Results {
		if r.CollectionName == term {
			return r, nil
		}
	}
	return resp.Results[0], nil
}

// httpGet fetches rawURL over GET, following redirects, and returns the 200
// body capped at maxBodyBytes. userAgent is set verbatim when non-empty (the
// Apple page needs a browser User-Agent). Non-200 statuses become an error
// and the https-only guard rejects any target not served over https or a
// loopback test server.
func httpGet(ctx context.Context, rawURL, userAgent string) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("podcast: invalid URL %q: %w", rawURL, err)
	}
	if err := checkHTTPS(u); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("podcast: building request: %w", err)
	}
	if userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("podcast: fetching %s: %w", u.Host, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("podcast: unexpected HTTP status %d from %s", resp.StatusCode, u.Host)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("podcast: reading %s: %w", u.Host, err)
	}
	return body, nil
}

// checkHTTPS rejects any target not served over https. Loopback hosts (used
// by httptest test servers) are the only plain-HTTP exception.
func checkHTTPS(u *url.URL) error {
	if u.Scheme == "https" {
		return nil
	}
	switch u.Hostname() {
	case "localhost", "127.0.0.1", "::1":
		return nil
	}
	return fmt.Errorf("podcast: refusing non-https URL %q", u.Redacted())
}
