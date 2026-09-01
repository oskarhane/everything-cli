package youtube

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/output"
	"github.com/oskarhane/google-cli/internal/subcommands/cmdtest"
	"github.com/oskarhane/google-cli/internal/youtube"
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

// fakeClient is the hermetic youtube.Client double: it serves a seeded
// Player and fails on demand. It never touches the network.
type fakeClient struct {
	player    *youtube.Player
	playerErr error
}

func (f *fakeClient) Player(context.Context, string) (*youtube.Player, error) {
	if f.playerErr != nil {
		return nil, f.playerErr
	}
	return f.player, nil
}

func (f *fakeClient) Transcript(context.Context, string) ([]youtube.Segment, error) {
	return nil, nil
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

// newLeafCmd builds the metadata leaf against a fake client, ready to
// execute.
func newLeafCmd(build func(*app.Config, youtube.Client) *cobra.Command, client youtube.Client, format string) *cobra.Command {
	return build(cmdtest.NewTestConfig(format), client)
}

// headerCells extracts the upper-cased header row of a table render, in
// column order.
func headerCells(t *testing.T, out string) []string {
	t.Helper()
	for _, line := range strings.Split(out, "\n") {
		if !strings.Contains(line, "VIDEO_ID") {
			continue
		}
		// StyleLight draws cells with the unicode box-drawing separator │.
		var cells []string
		for _, cell := range strings.Split(line, "│") {
			if s := strings.TrimSpace(cell); s != "" {
				cells = append(cells, s)
			}
		}
		return cells
	}
	t.Fatalf("no header row containing VIDEO_ID in table output:\n%s", out)
	return nil
}

// TestMetadataRenders: every format renders the eleven fields; table column
// order equals field order with upper-cased headers, JSON keys are exactly
// the snake_case field names, TOON shows the languages list.
func TestMetadataRenders(t *testing.T) {
	client := &fakeClient{player: seedPlayer()}

	tests := []struct {
		name   string
		format string
		check  func(t *testing.T, out string)
	}{
		{
			name:   "json",
			format: "json",
			check: func(t *testing.T, out string) {
				row, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
				require.True(t, ok, "expected a JSON object, got: %s", out)
				keys := cmdtest.JSONKeys(t, row)
				require.ElementsMatch(t, metadataFields, keys)
				cmdtest.RequireSnakeCase(t, keys)
			},
		},
		{
			name:   "table",
			format: "table",
			check: func(t *testing.T, out string) {
				require.Equal(t, []string{
					"VIDEO_ID", "TITLE", "CHANNEL", "CHANNEL_ID", "DURATION_SECONDS",
					"VIEW_COUNT", "PUBLISH_DATE", "UPLOAD_DATE", "CATEGORY",
					"DESCRIPTION", "AVAILABLE_LANGS",
				}, headerCells(t, out), "column order = field order, headers upper-cased")
				require.Contains(t, out, "Rick Astley - Never Gonna Give You Up")
				require.Contains(t, out, "[en en es ja]")
			},
		},
		{
			name:   "toon",
			format: "toon",
			check: func(t *testing.T, out string) {
				require.Contains(t, out, "video_id: "+videoID)
				require.Contains(t, out, "available_langs[#4]: en,en,es,ja")
				require.Contains(t, out, "Never Gonna Give You Up")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out := cmdtest.RunCmd(t, newLeafCmd(newMetadataCmd, client, tc.format), videoID)
			tc.check(t, out)
		})
	}
}

// TestMetadataValuesAcrossInputShapes: watch URL, youtu.be, shorts, and
// bare ID all resolve to the same video and render the same field values.
func TestMetadataValuesAcrossInputShapes(t *testing.T) {
	client := &fakeClient{player: seedPlayer()}

	for name, arg := range map[string]string{
		"watch URL":  "https://www.youtube.com/watch?v=" + videoID + "&feature=youtu.be",
		"youtu.be":   "https://youtu.be/" + videoID,
		"shorts URL": "https://www.youtube.com/shorts/" + videoID,
		"bare id":    videoID,
	} {
		t.Run(name, func(t *testing.T) {
			out := cmdtest.RunCmd(t, newLeafCmd(newMetadataCmd, client, "json"), arg)
			row, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
			require.True(t, ok, "expected a JSON object, got: %s", out)
			require.Equal(t, videoID, row["video_id"])
			require.Equal(t, "Rick Astley - Never Gonna Give You Up", row["title"])
			require.Equal(t, "Rick Astley", row["channel"])
			require.Equal(t, "UCuAXFkgsw1L7xaCfnd5JJOw", row["channel_id"])
			require.Equal(t, float64(213), row["duration_seconds"])
			require.Equal(t, float64(1600000000), row["view_count"])
			require.Equal(t, "2009-10-25", row["publish_date"])
			require.Equal(t, "2009-10-25", row["upload_date"])
			require.Equal(t, "Music", row["category"])
			require.Equal(t, "Rick Astley's official music video", row["description"])
		})
	}
}

// TestMetadataAvailableLangs: available_langs lists one entry per track —
// duplicates included, so both the human en and the ASR en appear — and an
// empty track list renders as an empty list, not null.
func TestMetadataAvailableLangs(t *testing.T) {
	t.Run("lists every track including duplicate human and asr en", func(t *testing.T) {
		out := cmdtest.RunCmd(t, newLeafCmd(newMetadataCmd, &fakeClient{player: seedPlayer()}, "json"), videoID)
		row, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
		require.True(t, ok, "expected a JSON object, got: %s", out)
		require.Equal(t, []any{"en", "en", "es", "ja"}, row["available_langs"])
	})

	t.Run("no tracks renders an empty list not null", func(t *testing.T) {
		p := seedPlayer()
		p.Tracks = nil
		out := cmdtest.RunCmd(t, newLeafCmd(newMetadataCmd, &fakeClient{player: p}, "json"), videoID)
		row, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
		require.True(t, ok, "expected a JSON object, got: %s", out)
		require.Equal(t, []any{}, row["available_langs"])
	})
}

// TestMetadataPlayerError: any Player failure fails RunE with an error that
// names the video ID, unplayable ones included.
func TestMetadataPlayerError(t *testing.T) {
	t.Run("unplayable names the video id", func(t *testing.T) {
		_, err := cmdtest.RunCmdErr(t,
			newLeafCmd(newMetadataCmd, &fakeClient{playerErr: youtube.ErrUnplayable}, "json"),
			"https://www.youtube.com/watch?v="+videoID)
		require.Contains(t, err.Error(), videoID)
		require.Contains(t, err.Error(), "video is not playable")
	})

	t.Run("any player error names the video id", func(t *testing.T) {
		_, err := cmdtest.RunCmdErr(t,
			newLeafCmd(newMetadataCmd, &fakeClient{playerErr: errors.New("boom")}, "json"),
			videoID)
		require.Contains(t, err.Error(), videoID)
		require.Contains(t, err.Error(), "boom")
	})
}

// TestMetadataInvalidVideoID: unrecognized references never reach the
// client; they fail with ErrBadVideoID.
func TestMetadataInvalidVideoID(t *testing.T) {
	client := &fakeClient{player: seedPlayer()}

	for name, arg := range map[string]string{
		"garbage":    "definitely-not-a-video-id",
		"wrong host": "https://example.com/watch?v=" + videoID,
		"missing id": "https://www.youtube.com/watch?v=",
		"bad path":   "https://www.youtube.com/videos/" + videoID,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := cmdtest.RunCmdErr(t, newLeafCmd(newMetadataCmd, client, "json"), arg)
			require.Contains(t, err.Error(), youtube.ErrBadVideoID.Error())
		})
	}
}

// TestMetadataRequiresExactlyOneArg: the leaf accepts exactly one
// positional.
func TestMetadataRequiresExactlyOneArg(t *testing.T) {
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newMetadataCmd, &fakeClient{player: seedPlayer()}, "json"))
	require.Contains(t, err.Error(), "accepts 1 arg")
}
