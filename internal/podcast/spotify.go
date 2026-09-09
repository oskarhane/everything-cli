package podcast

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
)

// spotifyEmbedPage is the base URL of Spotify's embed episode pages (which
// render server-side, no auth needed). It is a package-level seam: production
// points at the live https origin, while tests swap it for an httptest
// server.
var spotifyEmbedPage = "https://open.spotify.com/embed"

// nextDataScriptRe pulls the JSON payload out of the embed page's
// __NEXT_DATA__ script tag.
var nextDataScriptRe = regexp.MustCompile(`(?s)<script[^>]+id=["']__NEXT_DATA__["'][^>]*>(.*?)</script>`)

// next data payload shapes; only the entity's title fields are read.
type nextData struct {
	Props nextDataProps `json:"props"`
}

type nextDataProps struct {
	PageProps nextDataPageProps `json:"pageProps"`
}

type nextDataPageProps struct {
	State nextDataState `json:"state"`
}

type nextDataState struct {
	Data nextDataData `json:"data"`
}

type nextDataData struct {
	Entity nextDataEntity `json:"entity"`
}

type nextDataEntity struct {
	Subtitle string `json:"subtitle"`
	Title    string `json:"title"`
}

// ResolveSpotify resolves the metadata for a Spotify episode from its embed
// page: the show title (entity.subtitle) and episode title (entity.title) out
// of the __NEXT_DATA__ JSON, plus the show's RSS feed URL via iTunes search.
// A dead/stale episode URL (non-200 embed page) or a search miss yields
// ErrShowNotFound (wrapped with %w so errors.Is works).
func ResolveSpotify(ctx context.Context, ref Ref) (Meta, error) {
	if ref.Platform != PlatformSpotify {
		return Meta{}, fmt.Errorf("podcast: ResolveSpotify requires a spotify ref, got %q", ref.Platform)
	}
	pageURL := spotifyEmbedPage + "/episode/" + ref.EpisodeID
	body, err := httpGet(ctx, pageURL, "")
	if err != nil {
		return Meta{}, fmt.Errorf("%w: fetching Spotify embed page: %v", ErrShowNotFound, err)
	}
	showTitle, episodeTitle, err := extractNextData(string(body))
	if err != nil {
		return Meta{}, fmt.Errorf("%w: %v", ErrShowNotFound, err)
	}
	show, err := searchShow(ctx, showTitle)
	if err != nil {
		return Meta{}, fmt.Errorf("%w: iTunes search for %q: %v", ErrShowNotFound, showTitle, err)
	}
	return Meta{ShowTitle: showTitle, EpisodeTitle: episodeTitle, FeedURL: show.FeedURL}, nil
}

// extractNextData parses the embed page's __NEXT_DATA__ JSON and returns the
// episode's show title (entity.subtitle) and episode title (entity.title).
func extractNextData(page string) (showTitle, episodeTitle string, err error) {
	m := nextDataScriptRe.FindStringSubmatch(page)
	if m == nil {
		return "", "", fmt.Errorf("no __NEXT_DATA__ script on Spotify embed page")
	}
	var data nextData
	if err := json.Unmarshal([]byte(m[1]), &data); err != nil {
		return "", "", fmt.Errorf("decoding Spotify embed __NEXT_DATA__: %w", err)
	}
	entity := data.Props.PageProps.State.Data.Entity
	if entity.Subtitle == "" || entity.Title == "" {
		return "", "", fmt.Errorf("spotify embed page carries no entity title data")
	}
	return entity.Subtitle, entity.Title, nil
}
