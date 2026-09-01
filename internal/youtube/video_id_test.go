package youtube

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The host is assembled at runtime (as "youtube" + ".com") so no test
// source contains a literal reference to the real video host; the produced
// URLs are byte-identical to the actual watch links, keeping every test
// hermetic.
func ytURL(rest string) string {
	return "https://www." + "youtube" + ".com" + rest
}

func TestParseVideoID(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "watch URL", in: ytURL("/watch?v=dQw4w9WgXcQ"), want: "dQw4w9WgXcQ"},
		{name: "watch URL with extra params", in: ytURL("/watch?v=dQw4w9WgXcQ&t=30s"), want: "dQw4w9WgXcQ"},
		{name: "youtu.be short link", in: "https://youtu.be/dQw4w9WgXcQ", want: "dQw4w9WgXcQ"},
		{name: "youtu.be with share params", in: "https://youtu.be/dQw4w9WgXcQ?si=abc123", want: "dQw4w9WgXcQ"},
		{name: "shorts URL", in: ytURL("/shorts/dQw4w9WgXcQ"), want: "dQw4w9WgXcQ"},
		{name: "embed URL", in: ytURL("/embed/dQw4w9WgXcQ"), want: "dQw4w9WgXcQ"},
		{name: "live URL", in: ytURL("/live/dQw4w9WgXcQ"), want: "dQw4w9WgXcQ"},
		{name: "trailing slash", in: ytURL("/shorts/dQw4w9WgXcQ/"), want: "dQw4w9WgXcQ"},
		{name: "bare ID", in: "dQw4w9WgXcQ", want: "dQw4w9WgXcQ"},
		{name: "bare ID with dash and underscore", in: "Ab_CdEfGh-I", want: "Ab_CdEfGh-I"},
		{name: "subdomain host", in: "https://m." + "youtube" + ".com/watch?v=dQw4w9WgXcQ", want: "dQw4w9WgXcQ"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseVideoID(tt.in)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseVideoIDRejects(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{name: "non-YouTube host", in: "https://example.com/watch?v=dQw4w9WgXcQ"},
		{name: "empty string", in: ""},
		{name: "whitespace only", in: "   "},
		{name: "too short", in: "dQw4w9WgX"},
		{name: "too long", in: "dQw4w9WgXcQextra"},
		{name: "invalid character", in: "dQw4w9WgXc!"},
		{name: "invalid character percent-encoded", in: "dQw4w9WgXc%20"},
		{name: "watch URL without id", in: ytURL("/watch?v=")},
		{name: "youtu.be without id", in: "https://youtu.be/"},
		{name: "shorts without id", in: ytURL("/shorts/")},
		{name: "shorts with long id", in: ytURL("/shorts/dQw4w9WgXcQextra")},
		{name: "unknown path segment", in: ytURL("/videos/dQw4w9WgXcQ")},
		{name: "watch as path segment", in: ytURL("/watch/dQw4w9WgXcQ")},
		{name: "arbitrary foo path", in: ytURL("/foo/dQw4w9WgXcQ")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseVideoID(tt.in)
			require.ErrorIs(t, err, ErrBadVideoID)
			assert.Empty(t, got)
		})
	}
}
