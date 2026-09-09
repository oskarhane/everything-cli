package podcast

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
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
	body, err := get(ctx, itunesLookupEndpoint+"?"+q.Encode(), "")
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
	body, err := get(ctx, itunesSearchEndpoint+"?"+q.Encode(), "")
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
