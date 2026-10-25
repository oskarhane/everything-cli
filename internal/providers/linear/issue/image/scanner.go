package image

import (
	"regexp"
	"strings"

	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
)

// Source labels for where an image ref was found; the rendered values of
// the list leaf's source field.
const (
	sourceDescription = "description"
	sourceComment     = "comment"
)

// imageRef is one markdown image reference (`![alt](url)`) discovered in
// an issue description or comment body. Alt is the local filename's
// source; URL is the download target.
type imageRef struct {
	URL    string
	Alt    string
	Source string
}

// imageRefPattern matches the markdown image form Linear writes into
// descriptions and comments: alt text, destination URL, and an optional
// quoted title after the URL. The common form, not all of CommonMark.
var imageRefPattern = regexp.MustCompile(`!\[([^\]]*)\]\(([^)\s]*)(?:\s+"[^"]*")?\)`)

// scanImages returns every image ref in body, in document order, each
// tagged with source. A ref with an empty destination URL is skipped: it
// has nothing to list or download.
func scanImages(source, body string) []imageRef {
	matches := imageRefPattern.FindAllStringSubmatch(body, -1)
	if matches == nil {
		return nil
	}
	refs := make([]imageRef, 0, len(matches))
	for _, m := range matches {
		if m[2] == "" {
			continue
		}
		refs = append(refs, imageRef{URL: m[2], Alt: m[1], Source: source})
	}
	return refs
}

// discoverImages scans the issue description and then every comment body,
// deduplicating by URL across sources: the first occurrence wins (the
// description's alt text and source label), and later repeats are
// dropped.
func discoverImages(description string, comments []service.Comment) []imageRef {
	var refs []imageRef
	seen := map[string]bool{}
	add := func(found []imageRef) {
		for _, ref := range found {
			if seen[ref.URL] {
				continue
			}
			seen[ref.URL] = true
			refs = append(refs, ref)
		}
	}
	add(scanImages(sourceDescription, description))
	for i := range comments {
		add(scanImages(sourceComment, comments[i].Body))
	}
	return refs
}

// sanitizeFilename reduces a markdown alt text — untrusted issue content —
// to a bare local basename: control bytes (C0 and DEL) are removed, any
// path structure is cut to the last segment after / or \ (so ../
// traversal, absolute paths, and Windows drive paths cannot escape the
// output directory), and trailing dots and spaces are trimmed (so . and ..
// vanish). It returns "" when nothing survives; callers fall back.
func sanitizeFilename(alt string) string {
	// C0 control bytes and DEL never reach a filename.
	var b strings.Builder
	b.Grow(len(alt))
	for _, r := range alt {
		if r < 0x20 || r == 0x7F {
			continue
		}
		b.WriteRune(r)
	}
	seg := b.String()
	// Bare basename: keep only the last path segment on both separators.
	if i := strings.LastIndexAny(seg, `/\`); i >= 0 {
		seg = seg[i+1:]
	}
	return strings.TrimRight(strings.TrimSpace(seg), ". ")
}
