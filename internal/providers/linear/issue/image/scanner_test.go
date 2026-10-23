package image

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
)

// TestSanitizeFilename: alt text — untrusted issue content — reduces to a
// bare basename: no path separators, no .., no absolute path, no control
// bytes, and nothing left when it sanitizes to empty.
func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name string
		alt  string
		want string
	}{
		{"plain with extension", "login-form.png", "login-form.png"},
		{"plain without extension", "graph", "graph"},
		{"parent traversal", "../screenshot.png", "screenshot.png"},
		{"traversal without a file", "../", ""},
		{"dot dot", "..", ""},
		{"single dot", ".", ""},
		{"only dots", "...", ""},
		{"absolute path", "/etc/passwd.png", "passwd.png"},
		{"windows drive path", `C:\Users\oskar\shot.png`, "shot.png"},
		{"backslash traversal", `..\..\shot.png`, "shot.png"},
		{"empty alt", "", ""},
		{"control bytes removed", "shot\x00.png\n", "shot.png"},
		{"only control bytes", "\x01\x02", ""},
		{"trailing dots and spaces", "shot.png. ", "shot.png"},
		{"bare basename of a longer path", "shots/2026/graph.png", "graph.png"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, sanitizeFilename(tt.alt))
		})
	}
}

// TestScanImages: the scanner finds markdown image refs in document
// order, keeps link refs and empty destinations out, and tags every ref
// with its source.
func TestScanImages(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []imageRef
	}{
		{
			name: "two refs in order",
			body: "intro ![shot](u1.png) middle ![graph](u2.png) end",
			want: []imageRef{{URL: "u1.png", Alt: "shot", Source: "description"}, {URL: "u2.png", Alt: "graph", Source: "description"}},
		},
		{
			name: "a link is not an image",
			body: "[shot](u1.png) and ![real](u2.png)",
			want: []imageRef{{URL: "u2.png", Alt: "real", Source: "description"}},
		},
		{
			name: "ref with a title",
			body: `![shot](u1.png "the title") and ![plain](u2.png)`,
			want: []imageRef{{URL: "u1.png", Alt: "shot", Source: "description"}, {URL: "u2.png", Alt: "plain", Source: "description"}},
		},
		{
			name: "empty alt text",
			body: "![](u1.png)",
			want: []imageRef{{URL: "u1.png", Alt: "", Source: "description"}},
		},
		{
			name: "empty destination skipped",
			body: "![shot]() and ![real](u2.png)",
			want: []imageRef{{URL: "u2.png", Alt: "real", Source: "description"}},
		},
		{
			name: "no refs at all",
			body: "plain prose, no markdown",
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, scanImages("description", tt.body))
		})
	}
}

// TestScanImagesTagsSource: the source label comes from the caller, so
// the same body scans as a comment.
func TestScanImagesTagsSource(t *testing.T) {
	refs := scanImages(sourceComment, "![shot](u1.png)")

	require.Equal(t, []imageRef{{URL: "u1.png", Alt: "shot", Source: "comment"}}, refs)
}

// TestDiscoverImages: discovery scans the description first and every
// comment body after, deduplicating by URL across sources — the first
// occurrence (the description's) wins for both alt text and source.
func TestDiscoverImages(t *testing.T) {
	description := "![shot](u1.png) ![graph](u2.png)"
	comments := []service.Comment{
		{ID: "c1", Body: "repost: ![other alt](u2.png) plus ![fresh](u3.png)"},
		{ID: "c2", Body: "no images"},
	}

	refs := discoverImages(description, comments)

	want := []imageRef{
		{URL: "u1.png", Alt: "shot", Source: "description"},
		{URL: "u2.png", Alt: "graph", Source: "description"},
		{URL: "u3.png", Alt: "fresh", Source: "comment"},
	}
	require.Equal(t, want, refs)
}

// TestDiscoverImagesCommentOnly: an image that appears only in comments
// keeps the comment source and the comment's alt text.
func TestDiscoverImagesCommentOnly(t *testing.T) {
	refs := discoverImages("no images", []service.Comment{{ID: "c1", Body: "![late](u9.png)"}})

	require.Equal(t, []imageRef{{URL: "u9.png", Alt: "late", Source: "comment"}}, refs)
}

// TestDiscoverImagesEmpty: nothing to scan yields no refs, with or
// without comments.
func TestDiscoverImagesEmpty(t *testing.T) {
	require.Empty(t, discoverImages("", nil))
	require.Empty(t, discoverImages("no images", []service.Comment{{ID: "c1", Body: "prose"}}))
}
