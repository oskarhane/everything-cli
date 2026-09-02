package output

import (
	"fmt"
	"strings"
	"testing"

	"github.com/oskarhane/everything-cli/internal/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubResolutionSeams pins both detection seams for a test and restores them
// after. Every ResolveOutput test must pin both: the host runs inside an
// agent harness and may or may not have a TTY stdout.
func stubResolutionSeams(t *testing.T, isAgent, isTTY bool) {
	t.Helper()
	origAgent, origTTY := IsAgent, StdoutIsTerminal
	IsAgent = func() bool { return isAgent }
	StdoutIsTerminal = func() bool { return isTTY }
	t.Cleanup(func() {
		IsAgent = origAgent
		StdoutIsTerminal = origTTY
	})
}

func TestResolveOutput(t *testing.T) {
	for _, tc := range []struct {
		name       string
		formatFlag string
		isAgent    bool
		isTTY      bool
		want       Format
	}{
		{
			name:       "explicit json flag beats agent harness and tty",
			formatFlag: "json",
			isAgent:    true,
			isTTY:      true,
			want:       FormatJSON,
		},
		{
			name:       "explicit table flag beats agent harness",
			formatFlag: "table",
			isAgent:    true,
			isTTY:      false,
			want:       FormatTable,
		},
		{
			name:       "explicit toon flag beats tty",
			formatFlag: "toon",
			isAgent:    false,
			isTTY:      true,
			want:       FormatToon,
		},
		{
			name:    "agent harness without flag resolves to toon",
			isAgent: true,
			isTTY:   false,
			want:    FormatToon,
		},
		{
			name:    "agent harness without flag resolves to toon even on tty",
			isAgent: true,
			isTTY:   true,
			want:    FormatToon,
		},
		{
			name:  "tty stdout without flag resolves to table",
			isTTY: true,
			want:  FormatTable,
		},
		{
			name: "non-tty stdout without flag resolves to json",
			want: FormatJSON,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stubResolutionSeams(t, tc.isAgent, tc.isTTY)

			assert.Equal(t, tc.want, ResolveOutput(tc.formatFlag))
		})
	}
}

// TestResolveOutputAgentDetectionViaCLAUDECODE drives the real agent.Detect
// (not a stubbed IsAgent) with CLAUDECODE set, matching how a Claude Code
// harness actually resolves the format.
func TestResolveOutputAgentDetectionViaCLAUDECODE(t *testing.T) {
	stubResolutionSeams(t, false, false)
	orig := IsAgent
	IsAgent = agent.Detect
	t.Cleanup(func() { IsAgent = orig })
	t.Setenv("CLAUDECODE", "1")

	assert.Equal(t, FormatToon, ResolveOutput(""))
}

func TestResolveOutputInvalidFlag(t *testing.T) {
	for _, tc := range []struct {
		name       string
		formatFlag string
		isAgent    bool
		isTTY      bool
		want       Format
	}{
		{
			name:       "invalid flag falls through to json when non-tty",
			formatFlag: "yaml",
			isTTY:      false,
			want:       FormatJSON,
		},
		{
			name:       "invalid flag falls through to table when tty",
			formatFlag: "yaml",
			isTTY:      true,
			want:       FormatTable,
		},
		{
			name:       "invalid flag falls through to toon when agent harness",
			formatFlag: "pretty",
			isAgent:    true,
			want:       FormatToon,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stubResolutionSeams(t, tc.isAgent, tc.isTTY)

			var warned strings.Builder
			orig := Warnf
			Warnf = func(format string, args ...any) {
				_, _ = fmt.Fprintf(&warned, format, args...)
			}
			t.Cleanup(func() { Warnf = orig })

			assert.Equal(t, tc.want, ResolveOutput(tc.formatFlag))
			assert.Contains(t, warned.String(), tc.formatFlag,
				"invalid flag value should be surfaced in the warning")
		})
	}
}

func TestIsFormat(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  bool
	}{
		{"json", true},
		{"table", true},
		{"toon", true},
		{"", false},
		{"yaml", false},
		{"JSON", false},
	} {
		assert.Equal(t, tc.want, IsFormat(tc.value), "IsFormat(%q)", tc.value)
	}
}

func TestFormatValues(t *testing.T) {
	require.Equal(t, "json", string(FormatJSON))
	require.Equal(t, "table", string(FormatTable))
	require.Equal(t, "toon", string(FormatToon))
}
