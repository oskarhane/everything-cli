package youtube

import (
	"context"
	"testing"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/google-cli/internal/output"
	"github.com/oskarhane/google-cli/internal/subcommands/cmdtest"
	yt "github.com/oskarhane/google-cli/internal/youtube"
)

// transcriptFake is the hermetic youtube.Client double for the transcript
// leaf: it serves a canned player and transcript and records every call so
// tests can assert what the leaf dialed.
type transcriptFake struct {
	player        *yt.Player
	playerErr     error
	segments      []yt.Segment
	transcriptErr error

	playerID string // video ID passed to Player
	trackURL string // track BaseURL passed to Transcript
}

func (f *transcriptFake) Player(_ context.Context, videoID string) (*yt.Player, error) {
	f.playerID = videoID
	if f.playerErr != nil {
		return nil, f.playerErr
	}
	return f.player, nil
}

func (f *transcriptFake) Transcript(_ context.Context, trackURL string) ([]yt.Segment, error) {
	f.trackURL = trackURL
	if f.transcriptErr != nil {
		return nil, f.transcriptErr
	}
	return f.segments, nil
}

// seedTranscriptPlayer returns a canned player with two en tracks — a human
// one and a generated one — plus a generated de track, so --lang selection
// is observable through the BaseURL the client records.
func seedTranscriptPlayer() *yt.Player {
	return &yt.Player{
		VideoID: videoID,
		Title:   "Never Gonna Give You Up",
		Tracks: []yt.Track{
			{Lang: "en", Generated: false, BaseURL: "https://example.test/en-manual"},
			{Lang: "en", Generated: true, BaseURL: "https://example.test/en-asr"},
			{Lang: "de", Generated: true, BaseURL: "https://example.test/de-asr"},
		},
	}
}

// seedSegments returns the canned timed caption segments of the video.
func seedSegments() []yt.Segment {
	return []yt.Segment{
		{StartMS: 420, DurationMS: 1440, Text: "We're no strangers to love"},
		{StartMS: 2140, DurationMS: 1450, Text: "You know the rules and so do I"},
	}
}

// seedTranscriptFake returns a fake client serving the canned player and
// transcript.
func seedTranscriptFake() *transcriptFake {
	return &transcriptFake{player: seedTranscriptPlayer(), segments: seedSegments()}
}

// newCmd builds the transcript leaf against a fake client, ready to run.
func newCmd(fake *transcriptFake, format string) *cobra.Command {
	return newTranscriptCmd(cmdtest.NewTestConfig(format), fake)
}

// newCmdWithFs is newCmd but with a caller-supplied FS, for the --out tests.
func newCmdWithFs(fake *transcriptFake, format string, fs afero.Fs) *cobra.Command {
	cfg := cmdtest.NewTestConfig(format)
	cfg.Fs = fs
	return newTranscriptCmd(cfg, fake)
}

// stubTerminal overrides stdout terminal detection for the duration of a
// test, restoring the previous seam via t.Cleanup.
func stubTerminal(t *testing.T, isTTY bool) {
	t.Helper()
	old := output.StdoutIsTerminal
	output.StdoutIsTerminal = func() bool { return isTTY }
	t.Cleanup(func() { output.StdoutIsTerminal = old })
}

func TestTranscriptPipedStreamsPlainText(t *testing.T) {
	fake := seedTranscriptFake()
	stubTerminal(t, false)

	// A watch URL is accepted; the leaf parses it down to the bare ID.
	out := cmdtest.RunCmd(t, newCmd(fake, ""), "https://www.youtube.com/watch?v="+videoID)

	// Piped stdout means plain caption text: one line per segment, with no
	// table or JSON framing of any kind.
	require.Equal(t, "We're no strangers to love\nYou know the rules and so do I\n", out)
	require.NotContains(t, out, "video_id")
	require.NotContains(t, out, "VIDEO_ID")
	require.Equal(t, videoID, fake.playerID, "the URL's v= value is what reaches the client")
	require.Equal(t, "https://example.test/en-manual", fake.trackURL, "default --lang en picks the human en track over the generated one")
}

func TestTranscriptTTYStructuredTable(t *testing.T) {
	fake := seedTranscriptFake()
	stubTerminal(t, true)
	out := cmdtest.RunCmd(t, newCmd(fake, ""), videoID)

	// A terminal with no --format resolves to the table report.
	for _, header := range []string{"VIDEO_ID", "TITLE", "LANG", "IS_GENERATED", "SEGMENTS"} {
		require.Contains(t, out, header, "table headers render upper-case")
	}
	require.Contains(t, out, videoID)
	require.Contains(t, out, "Never Gonna Give You Up")
}

func TestTranscriptJSON(t *testing.T) {
	fake := seedTranscriptFake()
	// Package TestMain seeds non-TTY; an explicit --format renders JSON
	// regardless of the terminal.
	out := cmdtest.RunCmd(t, newCmd(fake, "json"), videoID)

	row, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok, "expected a JSON object, got: %s", out)
	keys := cmdtest.JSONKeys(t, row)
	require.ElementsMatch(t, []string{"video_id", "title", "lang", "is_generated", "segments"}, keys)
	cmdtest.RequireSnakeCase(t, keys)
	require.Equal(t, videoID, row["video_id"])
	require.Equal(t, "Never Gonna Give You Up", row["title"])
	require.Equal(t, "en", row["lang"])
	require.Equal(t, false, row["is_generated"])

	segs, ok := row["segments"].([]any)
	require.True(t, ok, "segments must be an array")
	require.Len(t, segs, 2)
	first, ok := segs[0].(map[string]any)
	require.True(t, ok)
	require.ElementsMatch(t, []string{"start_ms", "duration_ms", "text"}, cmdtest.JSONKeys(t, first))
	require.Equal(t, float64(420), first["start_ms"])
	require.Equal(t, float64(1440), first["duration_ms"])
	require.Equal(t, "We're no strangers to love", first["text"])
	require.Equal(t, "You know the rules and so do I", segs[1].(map[string]any)["text"])
}

func TestTranscriptExplicitFormatWinsOnTTY(t *testing.T) {
	fake := seedTranscriptFake()
	stubTerminal(t, true)
	out := cmdtest.RunCmd(t, newCmd(fake, "json"), videoID)

	row, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok, "explicit --format must beat TTY auto-detection")
	require.Contains(t, row, "video_id")
	require.NotContains(t, out, "VIDEO_ID", "no table headers under --format json")
}

func TestTranscriptRawForcesPlainTextOnTTY(t *testing.T) {
	fake := seedTranscriptFake()
	stubTerminal(t, true)
	out := cmdtest.RunCmd(t, newCmd(fake, ""), videoID, "--raw")

	require.Equal(t, "We're no strangers to love\nYou know the rules and so do I\n", out)
	require.NotContains(t, out, "VIDEO_ID", "--raw must bypass the table report")
}

func TestTranscriptOutWritesFile(t *testing.T) {
	fake := seedTranscriptFake()
	stubTerminal(t, false)
	fs := afero.NewMemMapFs()
	cmd := newCmdWithFs(fake, "", fs)

	out := cmdtest.RunCmd(t, cmd, videoID, "--out", "out/notes.txt")

	require.Empty(t, out, "--out sends the transcript to the file, not stdout")
	data, err := afero.ReadFile(fs, "out/notes.txt")
	require.NoError(t, err)
	require.Equal(t, "We're no strangers to love\nYou know the rules and so do I\n", string(data))
}

func TestTranscriptLangENSelectsHumanTrack(t *testing.T) {
	fake := seedTranscriptFake()
	cmdtest.RunCmd(t, newCmd(fake, "json"), videoID, "--lang", "en")

	// The fake offers two en tracks; the non-generated one must win.
	require.Equal(t, "https://example.test/en-manual", fake.trackURL)
}

func TestTranscriptLangSelectsGeneratedTrack(t *testing.T) {
	fake := seedTranscriptFake()
	out := cmdtest.RunCmd(t, newCmd(fake, "json"), videoID, "--lang", "de")

	// Only a generated de track exists, so it is selected and reported as
	// generated.
	require.Equal(t, "https://example.test/de-asr", fake.trackURL)
	row, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok)
	require.Equal(t, "de", row["lang"])
	require.Equal(t, true, row["is_generated"])
}

func TestTranscriptNoCaptionsErrorCarriesVideoID(t *testing.T) {
	fake := seedTranscriptFake()
	fake.player.Tracks = nil
	_, err := cmdtest.RunCmdErr(t, newCmd(fake, "json"), videoID)

	require.ErrorIs(t, err, yt.ErrNoCaptions)
	require.Contains(t, err.Error(), videoID)
	require.Contains(t, err.Error(), "no caption tracks")
}

func TestTranscriptEmptyTranscriptErrorCarriesVideoID(t *testing.T) {
	fake := seedTranscriptFake()
	fake.transcriptErr = yt.ErrEmptyTranscript
	_, err := cmdtest.RunCmdErr(t, newCmd(fake, "json"), videoID)

	require.ErrorIs(t, err, yt.ErrEmptyTranscript)
	require.Contains(t, err.Error(), videoID)
	require.Contains(t, err.Error(), "empty transcript")
}

func TestTranscriptPlayerErrorCarriesVideoID(t *testing.T) {
	fake := seedTranscriptFake()
	fake.playerErr = yt.ErrUnplayable
	_, err := cmdtest.RunCmdErr(t, newCmd(fake, "json"), videoID)

	require.ErrorIs(t, err, yt.ErrUnplayable)
	require.Contains(t, err.Error(), videoID)
	require.Contains(t, err.Error(), "not playable")
}

func TestTranscriptRejectsBadVideoReference(t *testing.T) {
	fake := seedTranscriptFake()
	for _, arg := range []string{
		"not-a-video-id",                  // not 11 characters
		"https://example.com/dQw4w9WgXcQ", // unrecognized host
		"https://youtu.be/",               // missing id
	} {
		_, err := cmdtest.RunCmdErr(t, newCmd(fake, ""), arg)
		require.ErrorIs(t, err, yt.ErrBadVideoID, "arg %q", arg)
	}
}

func TestTranscriptRequiresExactlyOneArg(t *testing.T) {
	_, err := cmdtest.RunCmdErr(t, newCmd(seedTranscriptFake(), ""))
	require.Contains(t, err.Error(), "accepts 1 arg")
}
