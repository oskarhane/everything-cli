package youtube

import (
	"net/url"
	"strings"
)

// videoIDLength is the canonical YouTube video ID length; IDs use a
// base64url alphabet without padding.
const videoIDLength = 11

// ParseVideoID extracts a canonical video ID from a YouTube watch URL
// (?v=...), a youtu.be short link, a /shorts/, /embed/, or /live/ path, or a
// bare 11-character video ID. Anything else yields ErrBadVideoID.
func ParseVideoID(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ErrBadVideoID
	}

	var id string
	if strings.Contains(s, "://") {
		u, err := url.Parse(s)
		if err != nil {
			return "", ErrBadVideoID
		}
		host := strings.ToLower(u.Host)
		switch {
		case host == "youtu.be":
			id = pathSegment(u.Path, 0)
		case host == "youtube.com" || strings.HasSuffix(host, ".youtube.com"):
			if strings.TrimRight(u.Path, "/") == "/watch" {
				id = u.Query().Get("v")
			} else {
				id = pathSegment(u.Path, 1)
			}
		default:
			return "", ErrBadVideoID
		}
	} else {
		id = s
	}

	if !isVideoID(id) {
		return "", ErrBadVideoID
	}
	return id, nil
}

// pathSegment returns the nth slash-separated segment of a URL path:
// youtu.be links carry the ID as segment 0 ("/AbCdEfGhIjK"), while
// /shorts/, /embed/, and /live/ links carry it as segment 1
// ("/shorts/AbCdEfGhIjK").
func pathSegment(p string, n int) string {
	p = strings.Trim(p, "/")
	if p == "" {
		return ""
	}
	segs := strings.Split(p, "/")
	if n >= len(segs) {
		return ""
	}
	return segs[n]
}

// isVideoID reports whether s has the canonical 11-character base64url
// shape of a YouTube video ID.
func isVideoID(s string) bool {
	if len(s) != videoIDLength {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}
