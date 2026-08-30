package label

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	gmail "google.golang.org/api/gmail/v1"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/output"
	"github.com/oskarhane/google-cli/internal/subcommands/gmail/service"
)

// TestMain neutralizes format auto-detection so the host's harness env and
// TTY cannot flip output expectations.
func TestMain(m *testing.M) {
	output.IsAgent = func() bool { return false }
	output.StdoutIsTerminal = func() bool { return false }
	os.Exit(m.Run())
}

// newTestConfig returns a config forcing the given explicit output format.
func newTestConfig(format string) *app.Config {
	return &app.Config{Format: format, Fs: afero.NewMemMapFs()}
}

// fakeService is the hermetic service.GmailService double: it serves seeded
// labels and records writes for assertions.
type fakeService struct {
	labels       []*gmail.Label // served by ListLabels and GetLabel-by-id
	getErr       error          // when set, GetLabel always fails
	created      *gmail.Label   // last CreateLabel request
	updatedID    string         // last UpdateLabel id
	updated      *gmail.Label   // last UpdateLabel request
	deleteCalled bool
	deletedID    string
	deleteErr    error
}

func (f *fakeService) ListLabels(context.Context) ([]*gmail.Label, error) {
	return f.labels, nil
}

func (f *fakeService) GetLabel(_ context.Context, id string) (*gmail.Label, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	for _, l := range f.labels {
		if l.Id == id {
			return l, nil
		}
	}
	return nil, fmt.Errorf("googleapi: Error 404: label %s not found", id)
}

func (f *fakeService) CreateLabel(_ context.Context, label *gmail.Label) (*gmail.Label, error) {
	f.created = label
	return &gmail.Label{Id: "Label_99", Name: label.Name, Type: "user"}, nil
}

func (f *fakeService) UpdateLabel(_ context.Context, id string, label *gmail.Label) (*gmail.Label, error) {
	f.updatedID = id
	f.updated = label
	return label, nil
}

func (f *fakeService) DeleteLabel(_ context.Context, id string) error {
	f.deleteCalled = true
	f.deletedID = id
	return f.deleteErr
}

// fakeNewSvc returns a serviceFunc handing out svc, so leaves run hermeti-
// cally with no network and no real account store.
func fakeNewSvc(svc *fakeService) serviceFunc {
	return func(context.Context) (service.GmailService, error) { return svc, nil }
}

// newLeafCmd builds a leaf against a fake service, ready to execute.
func newLeafCmd(build func(*app.Config, serviceFunc) *cobra.Command, svc *fakeService, format string) *cobra.Command {
	return build(newTestConfig(format), fakeNewSvc(svc))
}

// runCmd executes a leaf cmd with its positional args and flags, returning
// everything it wrote. args must NOT include the leaf's own name: SetArgs
// feeds a single command, not a command path.
func runCmd(t *testing.T, cmd *cobra.Command, args ...string) string {
	t.Helper()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)
	require.NoError(t, cmd.Execute())
	return buf.String()
}

// runCmdErr executes cmd expecting failure, returning the error and output.
func runCmdErr(t *testing.T, cmd *cobra.Command, args ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	require.Error(t, err)
	return buf.String(), err
}

// decodeJSON unmarshals one JSON document.
func decodeJSON(t *testing.T, s string) any {
	t.Helper()
	var v any
	require.NoError(t, json.Unmarshal([]byte(s), &v))
	return v
}

// jsonKeys returns the keys of a decoded JSON object.
func jsonKeys(t *testing.T, raw map[string]any) []string {
	t.Helper()
	keys := make([]string, 0, len(raw))
	for k := range raw {
		keys = append(keys, k)
	}
	return keys
}

// requireSnakeCase asserts every key is lower snake_case, the output casing
// contract for JSON and TOON.
func requireSnakeCase(t *testing.T, keys []string) {
	t.Helper()
	for _, k := range keys {
		require.Regexp(t, `^[a-z0-9_]+$`, k, "key %q must be lower snake_case", k)
	}
}

// seedLabels returns a small realistic label set.
func seedLabels() []*gmail.Label {
	return []*gmail.Label{
		{Id: "INBOX", Name: "INBOX", Type: "system", MessagesTotal: 12, MessagesUnread: 3, ThreadsTotal: 9},
		{Id: "Label_7", Name: "Travel", Type: "user", MessagesTotal: 5, MessagesUnread: 0, ThreadsTotal: 4},
	}
}
