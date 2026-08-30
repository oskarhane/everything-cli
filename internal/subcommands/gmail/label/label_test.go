package label

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/subcommands/cmdtest"
	"github.com/oskarhane/google-cli/internal/subcommands/gmail/service"
)

func TestNewCmdRegistersLeaves(t *testing.T) {
	cmd := NewCmd(cmdtest.NewTestConfig("json"), fakeNewSvc(&fakeService{}))

	want := []string{"list", "get", "create", "update", "delete"}
	var got []string
	for _, sub := range cmd.Commands() {
		got = append(got, sub.Name())
	}
	require.ElementsMatch(t, want, got)
}

func TestLeavesHaveExamples(t *testing.T) {
	// Every leaf needs a flush-left Example with at least two google-cli
	// invocations, and reads need a --format json one.
	tests := []struct {
		name  string
		build func(*app.Config, service.Dialer[service.GmailService]) *cobra.Command
		json  bool // read leaf: example must include --format json
	}{
		{"list", newListCmd, true},
		{"get", newGetCmd, true},
		{"create", newCreateCmd, false},
		{"update", newUpdateCmd, false},
		{"delete", newDeleteCmd, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := tt.build(cmdtest.NewTestConfig("json"), fakeNewSvc(&fakeService{}))
			require.NotEmpty(t, cmd.Example)
			require.Contains(t, cmd.Example, "# ")
			require.GreaterOrEqual(t, countInvocations(cmd.Example), 2)
			if tt.json {
				require.Contains(t, cmd.Example, "--format json")
			}
		})
	}
}

// countInvocations counts example lines invoking google-cli.
func countInvocations(example string) int {
	n := 0
	for _, line := range splitLines(example) {
		if len(line) >= len("google-cli") && line[:len("google-cli")] == "google-cli" {
			n++
		}
	}
	return n
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	return append(lines, s[start:])
}
