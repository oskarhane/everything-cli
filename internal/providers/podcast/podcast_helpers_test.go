package podcast

import (
	"context"
	"os"
	"testing"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
	podcastapi "github.com/oskarhane/everything-cli/internal/podcast"
	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

// canonicalURL is the Apple Podcasts episode URL seedEpisode() backs; it
// must parse to episodeID.
const (
	canonicalURL = "https://podcasts.apple.com/us/podcast/syntax/id123?i=456"
	episodeID    = "456"
)

// TestMain neutralizes format auto-detection so the host's harness env and
// TTY cannot flip output expectations.
func TestMain(m *testing.M) {
	output.IsAgent = func() bool { return false }
	output.StdoutIsTerminal = func() bool { return false }
	os.Exit(m.Run())
}

// fakeClient is the hermetic podcast.Client double for every leaf: it
// serves a seeded Episode and transcript segments, fails on demand, and
// records every call so tests can assert what a leaf dialed. It never
// touches the network.
type fakeClient struct {
	episode       *podcastapi.Episode
	episodeErr    error
	segments      []podcastapi.Segment
	transcriptErr error

	epRef string // episode ID passed to Episode
	epURL string // TranscriptURL passed to Transcript
}

func (f *fakeClient) Episode(_ context.Context, ref podcastapi.Ref) (*podcastapi.Episode, error) {
	f.epRef = ref.EpisodeID
	if f.episodeErr != nil {
		return nil, f.episodeErr
	}
	return f.episode, nil
}

func (f *fakeClient) Transcript(_ context.Context, url string) ([]podcastapi.Segment, error) {
	f.epURL = url
	if f.transcriptErr != nil {
		return nil, f.transcriptErr
	}
	return f.segments, nil
}

// seedEpisode returns a realistic episode resolution.
func seedEpisode() *podcastapi.Episode {
	return &podcastapi.Episode{
		Show:          "Syntax",
		Title:         "How to write great code",
		Platform:      string(podcastapi.PlatformApple),
		EpisodeID:     episodeID,
		TranscriptURL: "https://example.test/transcript",
	}
}

// seedSegments returns the canned timed transcript segments of the episode.
func seedSegments() []podcastapi.Segment {
	return []podcastapi.Segment{
		{StartMS: 420, DurationMS: 1440, Text: "Welcome back to the show"},
		{StartMS: 2140, DurationMS: 1450, Text: "Today we talk transcripts"},
	}
}

// seedTranscriptFake returns a fake client serving the canned episode and
// transcript.
func seedTranscriptFake() *fakeClient {
	return &fakeClient{episode: seedEpisode(), segments: seedSegments()}
}

// newLeafCmd builds a leaf against a fake client, ready to execute.
func newLeafCmd(build func(*app.Config, podcastapi.Client) *cobra.Command, client podcastapi.Client, format string) *cobra.Command {
	return build(cmdtest.NewTestConfig(format), client)
}

// newLeafCmdWithFs is newLeafCmd but with a caller-supplied FS, for the
// --out tests.
func newLeafCmdWithFs(build func(*app.Config, podcastapi.Client) *cobra.Command, client podcastapi.Client, format string, fs afero.Fs) *cobra.Command {
	cfg := cmdtest.NewTestConfig(format)
	cfg.Fs = fs
	return build(cfg, client)
}
