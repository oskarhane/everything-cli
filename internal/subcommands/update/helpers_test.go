package update

import (
	"bytes"
	"context"
	"errors"
	"os"
	"runtime"
	"testing"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
	updateapi "github.com/oskarhane/everything-cli/internal/update"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

// TestMain pins format auto-detection and terminal detection off: the host
// machine may run this suite inside an agent harness or a TTY, and neither
// may flip expectations.
func TestMain(m *testing.M) {
	output.IsAgent = func() bool { return false }
	output.StdoutIsTerminal = func() bool { return false }
	output.StdinIsTerminal = func() bool { return false }
	os.Exit(m.Run())
}

// newUpdateEnv returns a hermetic update environment: an in-memory FS, HOME
// pinned to a temp dir, and the root command with the update leaf mounted
// and stdout and stderr captured. Tests never touch the real binary path or
// network.
func newUpdateEnv(t *testing.T) (*app.Config, *cobra.Command, *bytes.Buffer, *bytes.Buffer) {
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

// execute runs the command tree with args and returns the captured stdout
// and the command's error.
func execute(t *testing.T, root *cobra.Command, out *bytes.Buffer, args ...string) (string, error) {
	t.Helper()
	out.Reset()
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

// fakeClient is a canned update.Client: it serves a fixed LatestRelease
// response and records Download calls so tests can assert nothing was
// downloaded.
type fakeClient struct {
	rel         *updateapi.Release
	latestErr   error
	downloadErr error
	downloads   []string
}

func (f *fakeClient) LatestRelease(ctx context.Context) (*updateapi.Release, error) {
	if f.latestErr != nil {
		return nil, f.latestErr
	}
	return f.rel, nil
}

func (f *fakeClient) Download(ctx context.Context, url string) ([]byte, error) {
	f.downloads = append(f.downloads, url)
	if f.downloadErr != nil {
		return nil, f.downloadErr
	}
	return nil, errors.New("fakeClient: Download must not be called")
}

// relFixture returns a canned release with the platform tarball and
// checksums assets.
func relFixture() *updateapi.Release {
	return &updateapi.Release{
		Tag: "v1.2.3",
		Assets: []updateapi.Asset{
			{Name: updateapi.AssetName(runtime.GOOS, runtime.GOARCH), URL: "https://example.com/tar.gz"},
			{Name: "checksums.txt", URL: "https://example.com/checksums"},
		},
	}
}

// skipHint is the skill_hint the real Run fills when the reinstall is
// skipped.
const skipHint = "run 'everything-cli skill install' to refresh the installed skill bundle"

// stubClient installs a fake releases client for the duration of the test.
func stubClient(t *testing.T, c updateapi.Client) {
	t.Helper()
	old := newClient
	newClient = func() updateapi.Client { return c }
	t.Cleanup(func() { newClient = old })
}

// stubRun replaces the runUpdate seam with a recording stub and restores it
// via t.Cleanup. The stub returns a canned success Result whose skill
// fields follow the real Run contract (skill_hint filled only when the
// reinstall is skipped) and records the computed Options.
func stubRun(t *testing.T) *[]updateapi.Options {
	t.Helper()
	var calls []updateapi.Options
	old := runUpdate
	runUpdate = func(ctx context.Context, client updateapi.Client, current string, opts updateapi.Options) (updateapi.Result, error) {
		calls = append(calls, opts)
		res := updateapi.Result{
			CurrentVersion:  current,
			LatestVersion:   "v1.2.3",
			UpdateAvailable: true,
			Updated:         true,
			BinaryPath:      "/usr/local/bin/everything-cli",
		}
		if opts.SkipSkillInstall {
			res.SkillHint = skipHint
			return res, nil
		}
		res.SkillInstalled = []string{"/home/u/.claude/skills/google-cli"}
		res.SkillVersion = "v1.2.3"
		return res, nil
	}
	t.Cleanup(func() { runUpdate = old })
	return &calls
}

// stubReadYesNo replaces the readYesNo seam with a stub answering answer
// for every call, recording the call count, and restoring the seam via
// t.Cleanup. Panicking-answer callers pass a stub that fails the test.
func stubReadYesNo(t *testing.T, answer bool) *int {
	t.Helper()
	var calls int
	old := readYesNo
	readYesNo = func() (bool, error) {
		calls++
		return answer, nil
	}
	t.Cleanup(func() { readYesNo = old })
	return &calls
}

// withStdinTerminal pins output.StdinIsTerminal for the duration of the
// test.
func withStdinTerminal(t *testing.T, v bool) {
	t.Helper()
	old := output.StdinIsTerminal
	output.StdinIsTerminal = func() bool { return v }
	t.Cleanup(func() { output.StdinIsTerminal = old })
}

// withAgent pins output.IsAgent for the duration of the test.
func withAgent(t *testing.T, v bool) {
	t.Helper()
	old := output.IsAgent
	output.IsAgent = func() bool { return v }
	t.Cleanup(func() { output.IsAgent = old })
}
