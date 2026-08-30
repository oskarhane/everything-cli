package freebusy

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	calendar "google.golang.org/api/calendar/v3"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/output"
	"github.com/oskarhane/google-cli/internal/subcommands/calendar/service"
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

// frozenNow anchors every relative window and default in the tests.
var frozenNow = time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)

// freezeNow pins the package clock for the test and restores it after.
func freezeNow(t *testing.T) {
	t.Helper()
	original := nowFunc
	nowFunc = func() time.Time { return frozenNow }
	t.Cleanup(func() { nowFunc = original })
}

// fakeFreeBusyService is the hermetic service.FreeBusyService double: it
// serves a seeded calendar list and freebusy response, and records the query
// params and list call for assertions.
type fakeFreeBusyService struct {
	entries    []*calendar.CalendarListEntry // served by ListCalendarList
	resp       *calendar.FreeBusyResponse    // served by QueryFreeBusy
	listCalled bool
	listErr    error
	params     []service.QueryFreeBusyParams
	queryErr   error
}

func (f *fakeFreeBusyService) ListCalendarList(context.Context) ([]*calendar.CalendarListEntry, error) {
	f.listCalled = true
	return f.entries, f.listErr
}

func (f *fakeFreeBusyService) QueryFreeBusy(_ context.Context, params service.QueryFreeBusyParams) (*calendar.FreeBusyResponse, error) {
	f.params = append(f.params, params)
	if f.queryErr != nil {
		return nil, f.queryErr
	}
	if f.resp == nil {
		return &calendar.FreeBusyResponse{}, nil
	}
	return f.resp, nil
}

// fakeNewSvc returns a service.Dialer[service.FreeBusyService] handing out svc, so the leaf runs
// hermetically with no network and no real account store.
func fakeNewSvc(svc *fakeFreeBusyService) service.Dialer[service.FreeBusyService] {
	return func(context.Context) (service.FreeBusyService, error) { return svc, nil }
}

// runCmd executes the freebusy cmd with its flags, returning everything it
// wrote.
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

// newCmd builds the freebusy leaf against a fake service, ready to execute.
func newCmd(svc *fakeFreeBusyService, format string) *cobra.Command {
	return NewCmd(newTestConfig(format), fakeNewSvc(svc))
}
