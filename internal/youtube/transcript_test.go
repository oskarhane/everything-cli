package youtube

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// wantSegments is the shape both trained XML fixtures must yield.
var wantSegments = []Segment{
	{StartMS: 1360, DurationMS: 1680, Text: "Hello there, it's me & you"},
	{StartMS: 3040, DurationMS: 2000, Text: "line one\nline two"},
}

// msXML is the modern timedtext shape: <p> with t/d attributes in
// milliseconds, character references, and a multiline segment.
const msXML = `<?xml version="1.0" encoding="utf-8" ?>
<transcript>
<p t="1360" d="1680">Hello there, it&#39;s me &amp; you</p>
<p t="3040" d="2000">line one
line two</p>
</transcript>`

// secXML is the legacy timedtext shape: <text> with start/dur attributes in
// seconds.
const secXML = `<?xml version="1.0" encoding="utf-8" ?>
<transcript>
<text start="1.36" dur="1.68">Hello there, it&#39;s me &amp; you</text>
<text start="3.04" dur="2.0">line one
line two</text>
</transcript>`

// fetchTranscript serves body over httptest and runs Transcript against it.
func fetchTranscript(t *testing.T, body string) ([]Segment, error) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return NewClient().Transcript(context.Background(), srv.URL+"/timedtext")
}

func TestTranscript(t *testing.T) {
	ctx := context.Background()

	t.Run("modern XML p t/d in milliseconds", func(t *testing.T) {
		got, err := fetchTranscript(t, msXML)
		require.NoError(t, err)
		assert.Equal(t, wantSegments, got)
	})

	t.Run("legacy XML text start/dur in seconds", func(t *testing.T) {
		got, err := fetchTranscript(t, secXML)
		require.NoError(t, err)
		assert.Equal(t, wantSegments, got)
	})

	t.Run("empty body is ErrEmptyTranscript", func(t *testing.T) {
		_, err := fetchTranscript(t, "")
		require.ErrorIs(t, err, ErrEmptyTranscript)
	})

	t.Run("whitespace-only body is ErrEmptyTranscript", func(t *testing.T) {
		_, err := fetchTranscript(t, "  \n\t ")
		require.ErrorIs(t, err, ErrEmptyTranscript)
	})

	t.Run("xml with no segments is ErrEmptyTranscript", func(t *testing.T) {
		_, err := fetchTranscript(t, "<transcript></transcript>")
		require.ErrorIs(t, err, ErrEmptyTranscript)
	})

	t.Run("non-200 names the status code", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "nope", http.StatusNotFound)
		}))
		t.Cleanup(srv.Close)

		_, err := NewClient().Transcript(ctx, srv.URL+"/timedtext")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "404")
	})
}
