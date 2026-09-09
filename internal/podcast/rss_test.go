package podcast

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pc20Feed is an RSS feed in the Podcasting 2.0 shape: items carry
// podcast:transcript tags in the podcastindex namespace. It mirrors a real
// show feed (e.g. 1.6 MB, many items) in miniature.
const pc20Feed = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:podcast="https://podcastindex.org/namespace/1.0">
<channel>
<title>My Show</title>
<link>https://example.com</link>
<language>en</language>
<item>
<title>Episode One</title>
<guid>1</guid>
<podcast:transcript url="https://transcripts.example/ep1.vtt" type="text/vtt" rel="captions" language="en"/>
</item>
<item>
<title>Episode Two</title>
<guid>2</guid>
<podcast:transcript url="https://transcripts.example/ep2.srt" type="application/srt" rel="captions" language="en"/>
<podcast:transcript url="https://transcripts.example/ep2.vtt" type="text/vtt" rel="captions" language="en"/>
</item>
<item>
<title>Episode Three</title>
<guid>3</guid>
<podcast:transcript url="https://transcripts.example/ep3.srt" type="application/x-subrip" rel="captions" language="fr"/>
</item>
</channel>
</rss>`

func TestParseFeed(t *testing.T) {
	f, err := parseFeed([]byte(pc20Feed))
	require.NoError(t, err)
	assert.Equal(t, "en", f.Language)
	require.Len(t, f.Items, 3)
	assert.Equal(t, "Episode One", f.Items[0].Title)
	require.Len(t, f.Items[0].Transcripts, 1)
	assert.Equal(t, "text/vtt", f.Items[0].Transcripts[0].Type)
	assert.Equal(t, "https://transcripts.example/ep1.vtt", f.Items[0].Transcripts[0].URL)
	assert.Equal(t, "en", f.Items[0].Transcripts[0].Language)

	// Episode Two carries both an SRT and a VTT transcript.
	require.Len(t, f.Items[1].Transcripts, 2)
	assert.Equal(t, "application/srt", f.Items[1].Transcripts[0].Type)
	assert.Equal(t, "text/vtt", f.Items[1].Transcripts[1].Type)

	// Episode Three carries the spec's application/x-subrip.
	require.Len(t, f.Items[2].Transcripts, 1)
	assert.Equal(t, "application/x-subrip", f.Items[2].Transcripts[0].Type)
	assert.Equal(t, "fr", f.Items[2].Transcripts[0].Language)
}

func TestParseFeedIgnoresItunesNamespace(t *testing.T) {
	const feed = `<?xml version="1.0"?>
<rss version="2.0" xmlns:itunes="http://www.itunes.com/dtds/podcast-1.0.dtd">
<channel><language>en</language>
<item><title>Real</title><itunes:title>Not This One</itunes:title></item>
</channel></rss>`
	f, err := parseFeed([]byte(feed))
	require.NoError(t, err)
	require.Len(t, f.Items, 1)
	assert.Equal(t, "Real", f.Items[0].Title)
}

func TestParseFeedNoTranscripts(t *testing.T) {
	const feed = `<rss version="2.0"><channel><title>s</title><item><title>Only Title</title></item></channel></rss>`
	f, err := parseFeed([]byte(feed))
	require.NoError(t, err)
	require.Len(t, f.Items, 1)
	assert.Empty(t, f.Items[0].Transcripts)
}

func TestSelectTranscript(t *testing.T) {
	tests := []struct {
		name     string
		tags     []transcriptTag
		feedLang string
		wantURL  string
		wantErr  bool
	}{
		{
			name:     "prefers VTT over SRT",
			tags:     []transcriptTag{{URL: "a.srt", Type: "application/srt", Language: "en"}, {URL: "b.vtt", Type: "text/vtt", Language: "en"}},
			feedLang: "en",
			wantURL:  "b.vtt",
		},
		{
			name:     "prefers language match within same type",
			tags:     []transcriptTag{{URL: "fr.vtt", Type: "text/vtt", Language: "fr"}, {URL: "en.vtt", Type: "text/vtt", Language: "en"}},
			feedLang: "en",
			wantURL:  "en.vtt",
		},
		{
			name:     "deterministic first when no language match",
			tags:     []transcriptTag{{URL: "first.vtt", Type: "text/vtt", Language: "fr"}, {URL: "second.vtt", Type: "text/vtt", Language: "de"}},
			feedLang: "en",
			wantURL:  "first.vtt",
		},
		{
			name:     "accepts real-world application/srt",
			tags:     []transcriptTag{{URL: "only.srt", Type: "application/srt", Language: "en"}},
			feedLang: "en",
			wantURL:  "only.srt",
		},
		{
			name:     "ignores unsupported types",
			tags:     []transcriptTag{{URL: "no.srt", Type: "text/plain", Language: "en"}, {URL: "yes.srt", Type: "application/srt", Language: "en"}},
			feedLang: "en",
			wantURL:  "yes.srt",
		},
		{
			name:     "no usable tag is ErrNoTranscript",
			tags:     []transcriptTag{{URL: "x", Type: "text/plain", Language: "en"}},
			feedLang: "en",
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := selectTranscript(tt.tags, tt.feedLang)
			if tt.wantErr {
				require.ErrorIs(t, err, ErrNoTranscript)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantURL, got.URL)
		})
	}
}

func TestFetchFeed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(pc20Feed))
	}))
	t.Cleanup(srv.Close)

	f, err := fetchFeed(context.Background(), srv.URL+"/feed.xml")
	require.NoError(t, err)
	require.Len(t, f.Items, 3)
	assert.Equal(t, "en", f.Language)
}

func TestFetchFeedRejectsNonHTTPSNonLoopback(t *testing.T) {
	_, err := fetchBytes(context.Background(), "http://example.com/podcast/rss")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "https")
}

func TestFetchFeedRejectsRedirectOffLoopback(t *testing.T) {
	// The initial request is fine (loopback), but the redirect target is a
	// non-loopback plain-HTTP host, which must be rejected on that hop.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://example.com/podcast/rss", http.StatusFound)
	}))
	t.Cleanup(srv.Close)

	_, err := fetchBytes(context.Background(), srv.URL+"/feed.xml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "redirect")
}

func TestFetchFeedNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	_, err := fetchBytes(context.Background(), srv.URL+"/feed.xml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

func TestFetchFeedExceedsCap(t *testing.T) {
	huge := strings.Repeat("x", maxFetchBytes+1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(huge))
	}))
	t.Cleanup(srv.Close)

	_, err := fetchBytes(context.Background(), srv.URL+"/feed.xml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds")
}
