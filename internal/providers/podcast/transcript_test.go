package podcast

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/output"
	podcastapi "github.com/oskarhane/everything-cli/internal/podcast"
	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

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

	out := cmdtest.RunCmd(t, newLeafCmd(newTranscriptCmd, fake, ""), canonicalURL)

	// Piped stdout means plain transcript text: one line per segment, with
	// no table or JSON framing of any kind.
	require.Equal(t, "Welcome back to the show\nToday we talk transcripts\n", out)
	require.NotContains(t, out, "episode_id")
	require.NotContains(t, out, "EPISODE_ID")
	require.Equal(t, episodeID, fake.epRef, "the URL's episode ID is what reaches the client")
	require.Equal(t, "https://example.test/transcript", fake.epURL, "the episode's TranscriptURL feeds Transcript")
}

func TestTranscriptTTYStructuredTable(t *testing.T) {
	fake := seedTranscriptFake()
	stubTerminal(t, true)
	out := cmdtest.RunCmd(t, newLeafCmd(newTranscriptCmd, fake, ""), canonicalURL)

	// A terminal with no --format resolves to the table report.
	for _, header := range []string{"SHOW", "EPISODE", "PLATFORM", "EPISODE_ID", "SEGMENTS"} {
		require.Contains(t, out, header, "table headers render upper-case")
	}
	require.Contains(t, out, "Syntax")
	require.Contains(t, out, "How to write great code")
	require.Contains(t, out, "2 segments · 00:02", "segments cell summarizes count + total duration instead of dumping structs")
}

func TestTranscriptJSON(t *testing.T) {
	fake := seedTranscriptFake()
	// Package TestMain seeds non-TTY; an explicit --format renders JSON
	// regardless of the terminal.
	out := cmdtest.RunCmd(t, newLeafCmd(newTranscriptCmd, fake, "json"), canonicalURL)

	row, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok, "expected a JSON object, got: %s", out)
	keys := cmdtest.JSONKeys(t, row)
	require.ElementsMatch(t, []string{"show", "episode", "platform", "episode_id", "segments"}, keys)
	cmdtest.RequireSnakeCase(t, keys)
	require.Equal(t, "Syntax", row["show"])
	require.Equal(t, "How to write great code", row["episode"])
	require.Equal(t, "apple", row["platform"])
	require.Equal(t, episodeID, row["episode_id"])

	segs, ok := row["segments"].([]any)
	require.True(t, ok, "segments must be an array")
	require.Len(t, segs, 2)
	first, ok := segs[0].(map[string]any)
	require.True(t, ok)
	require.ElementsMatch(t, []string{"start_ms", "duration_ms", "text"}, cmdtest.JSONKeys(t, first))
	require.Equal(t, float64(420), first["start_ms"])
	require.Equal(t, float64(1440), first["duration_ms"])
	require.Equal(t, "Welcome back to the show", first["text"])
	require.Equal(t, "Today we talk transcripts", segs[1].(map[string]any)["text"])
}

func TestTranscriptExplicitFormatWinsOnTTY(t *testing.T) {
	fake := seedTranscriptFake()
	stubTerminal(t, true)
	out := cmdtest.RunCmd(t, newLeafCmd(newTranscriptCmd, fake, "json"), canonicalURL)

	row, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok, "explicit --format must beat TTY auto-detection")
	require.Contains(t, row, "show")
	require.NotContains(t, out, "SHOW", "no table headers under --format json")
}

func TestTranscriptRawForcesPlainTextOnTTY(t *testing.T) {
	fake := seedTranscriptFake()
	stubTerminal(t, true)
	out := cmdtest.RunCmd(t, newLeafCmd(newTranscriptCmd, fake, ""), canonicalURL, "--raw")

	require.Equal(t, "Welcome back to the show\nToday we talk transcripts\n", out)
	require.NotContains(t, out, "SHOW", "--raw must bypass the table report")
}

func TestTranscriptRawSanitizesControlBytes(t *testing.T) {
	// Transcript text is creator-controlled: control bytes must be
	// neutralized on the raw path (which bypasses output.Print) so a
	// malicious transcript cannot inject ANSI styling or OSC 52 clipboard
	// writes into the user's terminal. StripControl replaces C0 bytes (and
	// DEL) with "?", keeping the per-line \n framing.
	tests := []struct {
		name string
		text string
		want string
	}{
		{"ANSI red sequence neutralized", "\x1b[31mred\x1b[0m", "?[31mred?[0m\n"},
		{"BEL byte neutralized", "ring\x07bell", "ring?bell\n"},
		{"plain text unchanged", "hello world", "hello world\n"},
		{"tab and newline preserved", "a\tb\nc", "a\tb\nc\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := seedTranscriptFake()
			fake.segments = []podcastapi.Segment{{StartMS: 0, DurationMS: 100, Text: tt.text}}
			stubTerminal(t, false)

			out := cmdtest.RunCmd(t, newLeafCmd(newTranscriptCmd, fake, ""), canonicalURL)

			require.Equal(t, tt.want, out, "C0 control bytes must render as ? on the piped raw path")
			require.NotContains(t, out, "\x1b", "no raw ESC byte may reach stdout")
			require.NotContains(t, out, "\x07", "no raw BEL byte may reach stdout")
		})
	}
}

func TestTranscriptOutSanitizesControlBytes(t *testing.T) {
	// The --out file sink shares writeRaw with stdout, so the same
	// sanitization applies — pin it explicitly rather than relying on the
	// shared-chokepoint structure.
	fake := seedTranscriptFake()
	fake.segments = []podcastapi.Segment{{StartMS: 0, DurationMS: 100, Text: "\x1b[31mred\x1b[0m"}}
	stubTerminal(t, false)
	fs := afero.NewMemMapFs()
	cmd := newLeafCmdWithFs(newTranscriptCmd, fake, "", fs)

	out := cmdtest.RunCmd(t, cmd, canonicalURL, "--out", "notes.txt")

	require.Empty(t, out)
	data, err := afero.ReadFile(fs, "notes.txt")
	require.NoError(t, err)
	require.Equal(t, "?[31mred?[0m\n", string(data), "--out file contents must be control-byte sanitized too")
}

func TestTranscriptOutWritesFile(t *testing.T) {
	fake := seedTranscriptFake()
	stubTerminal(t, false)
	fs := afero.NewMemMapFs()
	cmd := newLeafCmdWithFs(newTranscriptCmd, fake, "", fs)

	out := cmdtest.RunCmd(t, cmd, canonicalURL, "--out", "out/notes.txt")

	require.Empty(t, out, "--out sends the transcript to the file, not stdout")
	data, err := afero.ReadFile(fs, "out/notes.txt")
	require.NoError(t, err)
	require.Equal(t, "Welcome back to the show\nToday we talk transcripts\n", string(data))
}

func TestTranscriptOutWinsOnTTYAndOverFormat(t *testing.T) {
	// --out must never be silently ignored: it routes the plain text to the
	// file and silences stdout whether the terminal would auto-render a
	// report (TTY) or an explicit --format was given.
	tests := []struct {
		name   string
		format string
		tty    bool
	}{
		{name: "on a TTY", format: "", tty: true},
		{name: "with explicit --format", format: "json", tty: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := seedTranscriptFake()
			stubTerminal(t, tt.tty)
			fs := afero.NewMemMapFs()
			cmd := newLeafCmdWithFs(newTranscriptCmd, fake, tt.format, fs)

			out := cmdtest.RunCmd(t, cmd, canonicalURL, "--out", "notes.txt")

			require.Empty(t, out, "--out sends the transcript to the file, not stdout")
			data, err := afero.ReadFile(fs, "notes.txt")
			require.NoError(t, err)
			require.Equal(t, "Welcome back to the show\nToday we talk transcripts\n", string(data))
		})
	}
}

func TestTranscriptEpisodeErrorCarriesEpisodeID(t *testing.T) {
	fake := seedTranscriptFake()
	fake.episodeErr = podcastapi.ErrShowNotFound
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newTranscriptCmd, fake, "json"), canonicalURL)

	require.ErrorIs(t, err, podcastapi.ErrShowNotFound)
	require.Contains(t, err.Error(), episodeID)
	require.Contains(t, err.Error(), "show not found")
}

func TestTranscriptEmptyTranscriptErrorCarriesEpisodeID(t *testing.T) {
	fake := seedTranscriptFake()
	fake.transcriptErr = podcastapi.ErrEmptyTranscript
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newTranscriptCmd, fake, "json"), canonicalURL)

	require.ErrorIs(t, err, podcastapi.ErrEmptyTranscript)
	require.Contains(t, err.Error(), episodeID)
	require.Contains(t, err.Error(), "empty transcript")
}

func TestTranscriptRejectsBadURL(t *testing.T) {
	fake := seedTranscriptFake()
	for _, arg := range []string{
		"not-a-url", // no scheme
		"https://example.com/podcast/id123?i=456",          // unrecognized host
		"https://podcasts.apple.com/us/podcast/slug/id123", // missing ?i=
	} {
		_, err := cmdtest.RunCmdErr(t, newLeafCmd(newTranscriptCmd, fake, ""), arg)
		require.ErrorIs(t, err, podcastapi.ErrUnsupportedURL, "arg %q", arg)
	}
}

func TestTranscriptRequiresExactlyOneArg(t *testing.T) {
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newTranscriptCmd, seedTranscriptFake(), ""))
	require.Contains(t, err.Error(), "accepts 1 arg")
}
