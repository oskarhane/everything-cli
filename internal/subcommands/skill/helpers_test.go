package skill

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
	skillapi "github.com/oskarhane/everything-cli/internal/skill"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// TestMain pins format auto-detection off: the host machine may run this
// suite inside an agent harness or a TTY, and neither may flip expectations.
func TestMain(m *testing.M) {
	output.IsAgent = func() bool { return false }
	output.StdoutIsTerminal = func() bool { return false }
	os.Exit(m.Run())
}

// newSkillEnv returns a hermetic skill tree: an in-memory FS, HOME pinned to
// a temp dir (so agent path expansion is deterministic), and the skill tree
// mounted on a fresh root command with stdout and stderr captured. Tests
// never touch the real ~/.claude or ~/.config/google-cli.
func newSkillEnv(t *testing.T) (*app.Config, *cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", "")
	cfg := &app.Config{Fs: afero.NewMemMapFs()}
	root := app.NewRootCommand(cfg)
	root.AddCommand(NewCmd(cfg))
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(errOut)
	return cfg, root, out, errOut
}

// execute runs the command tree with args and returns the captured stdout,
// stderr, and the command's error. errOut may be nil when stderr is
// irrelevant.
func execute(t *testing.T, root *cobra.Command, out, errOut *bytes.Buffer, args ...string) (string, string, error) {
	t.Helper()
	out.Reset()
	if errOut != nil {
		errOut.Reset()
		root.SetErr(errOut)
	} else {
		root.SetErr(io.Discard)
	}
	root.SetArgs(args)
	err := root.Execute()
	var stderr string
	if errOut != nil {
		stderr = errOut.String()
	}
	return out.String(), stderr, err
}

// findAgent looks up a supported agent by id for seeding.
func findAgent(t *testing.T, name string) skillapi.Agent {
	t.Helper()
	a := skillapi.FindAgent(name)
	require.NotNil(t, a, "test references unknown agent %q", name)
	return *a
}

// seedAgentDir creates the DetectDir of the named agent on the test FS so
// DetectAgents sees it as installed.
func seedAgentDir(t *testing.T, fsys afero.Fs, name string) {
	t.Helper()
	p, ok := findAgent(t, name).DetectPath()
	require.True(t, ok)
	require.NoError(t, fsys.MkdirAll(p, 0o755))
}

// installDst is the on-FS destination of the bundle for a named agent.
func installDst(t *testing.T, name string) string {
	t.Helper()
	p, ok := findAgent(t, name).SkillsPath()
	require.True(t, ok)
	return filepath.Join(p, skillapi.SkillName)
}
