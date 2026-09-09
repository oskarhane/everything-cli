package podcast

import (
	"context"
	"fmt"
	"html"
	"net/url"
	"regexp"
)

// appleEpisodePage is the base URL of Apple Podcasts episode pages. It is a
// package-level seam: production points at the live https origin (kept
// https-only), while tests swap it for an httptest server.
var appleEpisodePage = "https://podcasts.apple.com"

// browserUserAgent mirrors a regular browser so the Apple episode page serves
// its server-rendered HTML (it answers a 301 first hit, then 200).
const browserUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124 Safari/537.36"

var (
	appleMetaTagRe  = regexp.MustCompile(`(?i)<meta\b[^>]*>`)
	appleOGTitleRe  = regexp.MustCompile(`(?i)\bproperty\s*=\s*["']og:title["']`)
	appleContentRe  = regexp.MustCompile(`(?i)\bcontent\s*=\s*"([^"]*)"`)
	appleContentSRe = regexp.MustCompile(`(?i)\bcontent\s*=\s*'([^']*)'`)
)

// ResolveApple resolves the metadata for an Apple Podcasts episode: the full
// episode title from the episode page's og:title meta tag (HTML-entity
// unescaped) and the show's RSS feed URL via the iTunes lookup by podcast ID.
// A lookup miss (resultCount 0) yields ErrShowNotFound.
func ResolveApple(ctx context.Context, ref Ref) (Meta, error) {
	if ref.Platform != PlatformApple {
		return Meta{}, fmt.Errorf("podcast: ResolveApple requires an apple ref, got %q", ref.Platform)
	}
	pageURL, err := applePageURL(ref)
	if err != nil {
		return Meta{}, err
	}
	body, err := get(ctx, pageURL, browserUserAgent)
	if err != nil {
		return Meta{}, fmt.Errorf("podcast: fetching Apple episode page: %w", err)
	}
	episodeTitle := extractOGTitle(string(body))
	if episodeTitle == "" {
		return Meta{}, fmt.Errorf("podcast: no og:title on Apple episode page for id %s", ref.PodcastID)
	}
	show, err := lookupShow(ctx, ref.PodcastID)
	if err != nil {
		return Meta{}, err
	}
	return Meta{ShowTitle: show.CollectionName, EpisodeTitle: episodeTitle, FeedURL: show.FeedURL}, nil
}

// applePageURL builds the https episode page URL for ref from the
// appleEpisodePage seam base.
func applePageURL(ref Ref) (string, error) {
	base, err := url.Parse(appleEpisodePage)
	if err != nil {
		return "", fmt.Errorf("podcast: invalid Apple page base: %w", err)
	}
	q := url.Values{}
	q.Set("i", ref.EpisodeID)
	u := *base
	u.Path = "/us/podcast/id" + url.PathEscape(ref.PodcastID)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// extractOGTitle returns the unescaped value of the page's og:title meta
// tag, or "" if the page carries none.
func extractOGTitle(page string) string {
	for _, tag := range appleMetaTagRe.FindAllString(page, -1) {
		if !appleOGTitleRe.MatchString(tag) {
			continue
		}
		if v := appleMetaContent(tag); v != "" {
			return html.UnescapeString(v)
		}
	}
	return ""
}

// appleMetaContent extracts a meta tag's content attribute value (accepting
// double- or single-quoted forms), or "" if absent.
func appleMetaContent(tag string) string {
	if m := appleContentRe.FindStringSubmatch(tag); m != nil {
		return m[1]
	}
	if m := appleContentSRe.FindStringSubmatch(tag); m != nil {
		return m[1]
	}
	return ""
}
