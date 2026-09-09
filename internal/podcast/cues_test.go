package podcast

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// wantCues is the shape both the VTT and SRT fixtures must yield.
var wantCues = []Segment{
	{StartMS: 1500, DurationMS: 1500, Text: "Hello from the first cue."},
	{StartMS: 4250, DurationMS: 1750, Text: "Second cue with\ntwo lines."},
	{StartMS: 7000, DurationMS: 2500, Text: "Encore."},
}

// vttFixture is a WebVTT body: dot-millis timestamps, a trailing optional
// header block and a NOTE section, and multiline cues.
const vttFixture = `WEBVTT

NOTE This is a comment block.

00:01.500 --> 00:03.000
Hello from the first cue.

00:04.250 --> 00:06.000
Second cue with
two lines.

Kind: captions
Language: en

00:07.000 --> 00:09.500
Encore.`

// srtFixture is a SubRip body: comma-millis timestamps and numeric cue
// identifiers.
const srtFixture = `1
00:00:01,500 --> 00:00:03,000
Hello from the first cue.

2
00:00:04,250 --> 00:00:06,000
Second cue with
two lines.

3
00:00:07,000 --> 00:00:09,500
Encore.`

func TestParseCuesSniffsFormatByMagic(t *testing.T) {
	t.Run("WebVTT via WEBVTT magic (dot-millis)", func(t *testing.T) {
		segs, err := parseCues([]byte(vttFixture))
		require.NoError(t, err)
		assert.Equal(t, wantCues, segs)
	})

	t.Run("SRT without VTT magic (comma-millis)", func(t *testing.T) {
		segs, err := parseCues([]byte(srtFixture))
		require.NoError(t, err)
		assert.Equal(t, wantCues, segs)
	})

	t.Run("declared mime is ignored, magic wins", func(t *testing.T) {
		// A body declared application/srt but carrying the WEBVTT magic (and
		// dot-millis) must parse as VTT.
		segs, err := parseCues([]byte(vttFixture))
		require.NoError(t, err)
		assert.Equal(t, wantCues, segs)
	})
}

func TestParseCuesOptionalHours(t *testing.T) {
	const vtt = "WEBVTT\n\n01:02:03.250 --> 01:02:05.000\nOver an hour in."
	segs, err := parseCues([]byte(vtt))
	require.NoError(t, err)
	require.Len(t, segs, 1)
	assert.Equal(t, int64(3723250), segs[0].StartMS)
	assert.Equal(t, int64(1750), segs[0].DurationMS)
	assert.Equal(t, "Over an hour in.", segs[0].Text)
}

func TestParseCuesEdgeCases(t *testing.T) {
	t.Run("empty body yields empty", func(t *testing.T) {
		segs, err := parseCues(nil)
		require.NoError(t, err)
		assert.Empty(t, segs)
	})

	t.Run("header only yields empty", func(t *testing.T) {
		segs, err := parseCues([]byte("WEBVTT\n\nKind: captions\nLanguage: en\n"))
		require.NoError(t, err)
		assert.Empty(t, segs)
	})

	t.Run("cue with no text is dropped", func(t *testing.T) {
		segs, err := parseCues([]byte("WEBVTT\n\n00:01.000 --> 00:02.000\n\n00:02.000 --> 00:03.000\nSpoken."))
		require.NoError(t, err)
		require.Len(t, segs, 1)
		assert.Equal(t, "Spoken.", segs[0].Text)
		assert.Equal(t, int64(2000), segs[0].StartMS)
	})

	t.Run("CRLF line endings", func(t *testing.T) {
		body := "WEBVTT\r\n\r\n00:01.000 --> 00:02.000\r\nHi there.\r\n"
		segs, err := parseCues([]byte(body))
		require.NoError(t, err)
		require.Len(t, segs, 1)
		assert.Equal(t, "Hi there.", segs[0].Text)
	})

	t.Run("garbage yields empty", func(t *testing.T) {
		segs, err := parseCues([]byte("this is not a transcript at all"))
		require.NoError(t, err)
		assert.Empty(t, segs)
	})
}

func TestParseTimestamp(t *testing.T) {
	cases := []struct {
		in      string
		wantMS  int64
		wantErr bool
	}{
		{in: "00:01.500", wantMS: 1500},
		{in: "01:05.000", wantMS: 65000},
		{in: "1:02:03.250", wantMS: 3723250},
		{in: "00:00:01,500", wantMS: 1500}, // SRT comma-millis
		{in: "0:00:04,250", wantMS: 4250},
		{in: "1:02:03,999", wantMS: 3723999},
		{in: "nope", wantErr: true},
		{in: "", wantErr: true},
	}
	for _, tt := range cases {
		got, ok := parseTimestamp(tt.in)
		if tt.wantErr {
			require.False(t, ok)
			continue
		}
		require.True(t, ok)
		assert.Equal(t, tt.wantMS, got)
	}
}
