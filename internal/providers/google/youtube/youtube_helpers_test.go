package youtube

import (
	"context"
	"os"
	"testing"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
	"github.com/oskarhane/everything-cli/internal/youtube"
)

// videoID is the canonical ID seedPlayer() reports; every input shape
// (watch URL, youtu.be, shorts, bare ID) must resolve to it.
const videoID = "dQw4w9WgXcQ"

// TestMain neutralizes format auto-detection so the host's harness env and
// TTY cannot flip output expectations.
func TestMain(m *testing.M) {
	output.IsAgent = func() bool { return false }
	output.StdoutIsTerminal = func() bool { return false }
	os.Exit(m.Run())
}

// fakeClient is the hermetic youtube.Client double for every leaf: it
// serves a seeded Player and transcript segments, fails on demand, and
// records every call so tests can assert what a leaf dialed. It never
// touches the network.
type fakeClient struct {
	player        *youtube.Player
	playerErr     error
	segments      []youtube.Segment
	transcriptErr error

	playerID string // video ID passed to Player
	trackURL string // track BaseURL passed to Transcript
}

func (f *fakeClient) Player(_ context.Context, videoID string) (*youtube.Player, error) {
	f.playerID = videoID
	if f.playerErr != nil {
		return nil, f.playerErr
	}
	return f.player, nil
}

func (f *fakeClient) Transcript(_ context.Context, trackURL string) ([]youtube.Segment, error) {
	f.trackURL = trackURL
	if f.transcriptErr != nil {
		return nil, f.transcriptErr
	}
	return f.segments, nil
}

// seedPlayer returns a realistic player response: two "en" tracks (human +
// ASR), plus es and ja, so available_langs must carry the duplicate.
func seedPlayer() *youtube.Player {
	return &youtube.Player{
		VideoID:       videoID,
		Title:         "Rick Astley - Never Gonna Give You Up",
		Author:        "Rick Astley",
		ChannelID:     "UCuAXFkgsw1L7xaCfnd5JJOw",
		LengthSeconds: 213,
		ViewCount:     1600000000,
		PublishDate:   "2009-10-25",
		UploadDate:    "2009-10-25",
		Category:      "Music",
		Description:   "Rick Astley's official music video",
		Tracks: []youtube.Track{
			{Lang: "en", Generated: false, BaseURL: "https://www.youtube.com/api/timedtext?lang=en"},
			{Lang: "en", Generated: true, BaseURL: "https://www.youtube.com/api/timedtext?lang=en&kind=asr"},
			{Lang: "es", Generated: true, BaseURL: "https://www.youtube.com/api/timedtext?lang=es"},
			{Lang: "ja", Generated: false, BaseURL: "https://www.youtube.com/api/timedtext?lang=ja"},
		},
	}
}

// seedTranscriptPlayer returns a canned player with two en tracks — a human
// one and a generated one — plus a generated de track, so --lang selection
// is observable through the BaseURL the client records.
func seedTranscriptPlayer() *youtube.Player {
	return &youtube.Player{
		VideoID: videoID,
		Title:   "Never Gonna Give You Up",
		Tracks: []youtube.Track{
			{Lang: "en", Generated: false, BaseURL: "https://example.test/en-manual"},
			{Lang: "en", Generated: true, BaseURL: "https://example.test/en-asr"},
			{Lang: "de", Generated: true, BaseURL: "https://example.test/de-asr"},
		},
	}
}

// seedSegments returns the canned timed caption segments of the video.
func seedSegments() []youtube.Segment {
	return []youtube.Segment{
		{StartMS: 420, DurationMS: 1440, Text: "We're no strangers to love"},
		{StartMS: 2140, DurationMS: 1450, Text: "You know the rules and so do I"},
	}
}

// seedTranscriptFake returns a fake client serving the canned player and
// transcript.
func seedTranscriptFake() *fakeClient {
	return &fakeClient{player: seedTranscriptPlayer(), segments: seedSegments()}
}

// newLeafCmd builds a leaf against a fake client, ready to execute.
func newLeafCmd(build func(*app.Config, youtube.Client) *cobra.Command, client youtube.Client, format string) *cobra.Command {
	return build(cmdtest.NewTestConfig(format), client)
}

// newLeafCmdWithFs is newLeafCmd but with a caller-supplied FS, for the
// --out tests.
func newLeafCmdWithFs(build func(*app.Config, youtube.Client) *cobra.Command, client youtube.Client, format string, fs afero.Fs) *cobra.Command {
	cfg := cmdtest.NewTestConfig(format)
	cfg.Fs = fs
	return build(cfg, client)
}
