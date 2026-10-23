package file

import (
	"context"
	"io"
	"os"
	"testing"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

// TestMain neutralizes format auto-detection so the host's harness env and
// TTY cannot flip output expectations.
func TestMain(m *testing.M) {
	output.IsAgent = func() bool { return false }
	output.StdoutIsTerminal = func() bool { return false }
	os.Exit(m.Run())
}

// TestFileNewCmdRegistersLeaves: the `linear file` parent wires every leaf
// as one AddCommand line each.
func TestFileNewCmdRegistersLeaves(t *testing.T) {
	cmd := NewCmd(cmdtest.NewTestConfig("json"), fakeNewSvc(&fakeService{}))

	require.Equal(t, "file", cmd.Name())
	var names []string
	for _, sub := range cmd.Commands() {
		names = append(names, sub.Name())
	}
	require.ElementsMatch(t, []string{"download"}, names)
}

// fakeService is the hermetic service.FileService double: it records the URL
// it receives, streams the seeded content, and fails every call when err is
// set; with streamErr set it writes the content first and fails after, so a
// download dies mid-stream with bytes already on the writer.
// DownloadFileMeta reports the byte count it wrote; the file leaf
// never reads the Content-Type.
type fakeService struct {
	err       error
	streamErr error
	gotURL    string
	content   []byte
}

func (f *fakeService) DownloadFileTo(ctx context.Context, fileURL string, w io.Writer) error {
	_, err := f.DownloadFileMeta(ctx, fileURL, w)
	return err
}

func (f *fakeService) DownloadFileMeta(_ context.Context, fileURL string, w io.Writer) (service.FileMeta, error) {
	f.gotURL = fileURL
	if f.err != nil {
		return service.FileMeta{}, f.err
	}
	n, err := w.Write(f.content)
	if f.streamErr != nil {
		return service.FileMeta{Bytes: int64(n)}, f.streamErr
	}
	return service.FileMeta{Bytes: int64(n)}, err
}

// fakeNewSvc hands out svc so leaves run hermetically with no network and no
// real account store.
func fakeNewSvc(svc *fakeService) service.Dialer[service.FileService] {
	return func(context.Context) (service.FileService, error) { return svc, nil }
}

// newLeafCmd builds a leaf against a fake service, ready to execute.
func newLeafCmd(build func(*app.Config, service.Dialer[service.FileService]) *cobra.Command, svc *fakeService, format string) *cobra.Command {
	return build(cmdtest.NewTestConfig(format), fakeNewSvc(svc))
}

// newLeafCmdWithFs builds a leaf against a fake service on the given FS,
// for --out assertions that read the file back.
func newLeafCmdWithFs(build func(*app.Config, service.Dialer[service.FileService]) *cobra.Command, svc *fakeService, format string, fs afero.Fs) *cobra.Command {
	return build(&app.Config{Format: format, Fs: fs}, fakeNewSvc(svc))
}

// readAll reads one file out of an in-memory FS.
func readAll(t *testing.T, fs afero.Fs, path string) []byte {
	t.Helper()
	data, err := afero.ReadFile(fs, path)
	require.NoError(t, err)
	return data
}
