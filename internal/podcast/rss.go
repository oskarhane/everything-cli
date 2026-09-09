package podcast

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// podcast5 namespace URIs for the Podcasting 2.0 namespace that declares the
// <podcast:transcript> elements. Two URIs are seen in the wild: the canonical
// spec URI (https://podcastindex.org/namespace/1.0) and the older GitHub-docs
// URI still used by major feeds (e.g. Podcasting 2.0's own feed).
var podcastNS = map[string]bool{
	"https://podcastindex.org/namespace/1.0":                                      true,
	"https://github.com/Podcastindex-org/podcast-namespace/blob/main/docs/1.0.md": true,
}

// inPodcastNS reports whether space is one of the accepted Podcasting 2.0
// namespace URIs.
func inPodcastNS(space string) bool { return podcastNS[space] }

// feed is the subset of an RSS 2.0 podcast feed the podcast client needs: the
// channel language (used to prefer a matching transcript) and the channel's
// items together with their podcast:transcript tags.
type feed struct {
	Language string
	Items    []feedItem
}

// feedItem is one <item> of a podcast feed: its title plus any
// podcast:transcript elements it declares.
type feedItem struct {
	Title       string
	Transcripts []transcriptTag
}

// transcriptTag is one <podcast:transcript> element's attributes.
type transcriptTag struct {
	URL      string
	Type     string
	Rel      string
	Language string
}

// fetchFeed fetches and parses a podcast RSS feed reachable at feedURL.
func fetchFeed(ctx context.Context, feedURL string) (*feed, error) {
	data, err := get(ctx, feedURL, "")
	if err != nil {
		return nil, err
	}
	f, err := parseFeed(data)
	if err != nil {
		return nil, fmt.Errorf("podcast: parsing feed %s: %w", feedURL, err)
	}
	return f, nil
}

// parseFeed streams an RSS 2.0 feed via encoding/xml, collecting the channel
// language, per-item titles, and podcast:transcript tags. Feeds can be large
// (well over a megabyte), so parsing is streaming rather than buffered into a
// DOM. Only the default-namespace <item>/<title>/<language> elements are
// captured, so itunes-namespaced lookalikes (<itunes:title>) are ignored.
func parseFeed(data []byte) (*feed, error) {
	body := bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF}) // allow a UTF-8 BOM
	dec := xml.NewDecoder(bytes.NewReader(body))
	f := &feed{}
	var cur *feedItem
	var curTitle, curLang *string
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch {
			case t.Name.Local == "item" && t.Name.Space == "":
				cur = &feedItem{}
			case t.Name.Local == "title" && t.Name.Space == "":
				if cur != nil {
					curTitle = &cur.Title
				} else {
					curTitle = nil // channel title, not needed
				}
			case t.Name.Local == "language" && t.Name.Space == "":
				if cur == nil && curLang == nil {
					curLang = &f.Language
				} else {
					curLang = nil
				}
			case t.Name.Local == "transcript" && inPodcastNS(t.Name.Space) && cur != nil:
				cur.Transcripts = append(cur.Transcripts, transcriptTagOf(t))
			}
		case xml.CharData:
			if curTitle != nil {
				*curTitle += string(t)
			}
			if curLang != nil {
				*curLang += string(t)
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "item":
				if cur != nil {
					f.Items = append(f.Items, *cur)
				}
				cur = nil
			case "title":
				curTitle = nil
			case "language":
				curLang = nil
			}
		}
	}
	for i := range f.Items {
		f.Items[i].Title = strings.TrimSpace(f.Items[i].Title)
	}
	f.Language = strings.TrimSpace(f.Language)
	return f, nil
}

// transcriptTagOf reads the url/type/rel/language attributes of a
// podcast:transcript start element.
func transcriptTagOf(se xml.StartElement) transcriptTag {
	var tag transcriptTag
	for _, a := range se.Attr {
		switch a.Name.Local {
		case "url":
			tag.URL = a.Value
		case "type":
			tag.Type = a.Value
		case "rel":
			tag.Rel = a.Value
		case "language":
			tag.Language = a.Value
		}
	}
	return tag
}

// usableTranscriptType ranks an accepted transcript MIME type: VTT is
// preferred over SRT. ok is false for any unsupported MIME type. The accepted
// set covers the Podcasting 2.0 spec (text/vtt, application/x-subrip) and the
// real-world application/srt variance.
func usableTranscriptType(mime string) (rank int, ok bool) {
	switch strings.ToLower(strings.TrimSpace(mime)) {
	case "text/vtt":
		return 0, true
	case "application/x-subrip", "application/srt":
		return 1, true
	default:
		return 0, false
	}
}

// selectTranscript chooses one transcript tag for an episode. VTT is
// preferred over SRT; among equal-ranked tags a tag whose language matches the
// feed language wins; ties resolve to deterministic first-match. No usable tag
// yields ErrNoTranscript.
func selectTranscript(tags []transcriptTag, feedLanguage string) (transcriptTag, error) {
	var best transcriptTag
	bestRank, bestLang := 0, 1
	found := false
	feedLanguage = strings.ToLower(strings.TrimSpace(feedLanguage))
	for i := range tags {
		rank, ok := usableTranscriptType(tags[i].Type)
		if !ok {
			continue
		}
		lang := 1
		if feedLanguage != "" && strings.ToLower(strings.TrimSpace(tags[i].Language)) == feedLanguage {
			lang = 0
		}
		if !found || rank < bestRank || (rank == bestRank && lang < bestLang) {
			best = tags[i]
			bestRank, bestLang = rank, lang
			found = true
		}
	}
	if !found {
		return transcriptTag{}, ErrNoTranscript
	}
	return best, nil
}
