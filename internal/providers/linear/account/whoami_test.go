package account

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
)

// fakeViewerService stands in for the viewer seam: it returns a canned
// viewer (or a failure) without touching the network, and records that the
// leaf actually dialed.
type fakeViewerService struct {
	viewer  service.Viewer
	err     error
	invoked bool
}

func (f *fakeViewerService) GetViewer(context.Context) (service.Viewer, error) {
	f.invoked = true
	return f.viewer, f.err
}

// viewerDialerFor wires the fake into the account tree's whoami seam. The
// strategy factory is irrelevant on this path — whoami resolves its account
// through the dialer — so these tests reuse the production API-key strategy
// rather than minting a second fake.
func viewerDialerFor(svc *fakeViewerService) service.Dialer[service.ViewerService] {
	return func(context.Context) (service.ViewerService, error) { return svc, nil }
}

func TestWhoamiJSONPrintsViewerIdentity(t *testing.T) {
	svc := &fakeViewerService{viewer: service.Viewer{
		ID:    "usr_1",
		Name:  "Oskar Hane",
		Email: "oskar@example.com",
	}}
	_, root, out := newAccountEnv(t, realStrategy(t), viewerDialerFor(svc))

	stdout, err := execute(t, root, out, "account", "whoami", "--format", "json")
	require.NoError(t, err)
	require.True(t, svc.invoked)

	m, ok := decodeJSONMap(t, stdout)
	require.True(t, ok, "whoami prints a single object, not an array")
	require.Equal(t, "usr_1", m["id"])
	require.Equal(t, "Oskar Hane", m["name"])
	require.Equal(t, "oskar@example.com", m["email"])
}

func TestWhoamiTableUpperCasesHeaders(t *testing.T) {
	svc := &fakeViewerService{viewer: service.Viewer{
		ID:    "usr_1",
		Name:  "Oskar Hane",
		Email: "oskar@example.com",
	}}
	_, root, out := newAccountEnv(t, realStrategy(t), viewerDialerFor(svc))

	stdout, err := execute(t, root, out, "account", "whoami", "--format", "table")
	require.NoError(t, err)
	require.Contains(t, stdout, "ID")
	require.Contains(t, stdout, "NAME")
	require.Contains(t, stdout, "EMAIL")
	require.Contains(t, stdout, "oskar@example.com")
}

func TestWhoamiToonPrintsViewerIdentity(t *testing.T) {
	svc := &fakeViewerService{viewer: service.Viewer{
		ID:    "usr_1",
		Name:  "Oskar Hane",
		Email: "oskar@example.com",
	}}
	_, root, out := newAccountEnv(t, realStrategy(t), viewerDialerFor(svc))

	stdout, err := execute(t, root, out, "account", "whoami", "--format", "toon")
	require.NoError(t, err)
	require.Contains(t, stdout, "usr_1")
	require.Contains(t, stdout, "Oskar Hane")
	require.Contains(t, stdout, "oskar@example.com")
}

// TestWhoamiRejectsArguments pins the zero-positional convention: whoami
// takes no arguments, extras are an error, not silently ignored.
func TestWhoamiRejectsArguments(t *testing.T) {
	svc := &fakeViewerService{}
	_, root, out := newAccountEnv(t, realStrategy(t), viewerDialerFor(svc))

	_, err := execute(t, root, out, "account", "whoami", "extra")
	require.Error(t, err)
	require.False(t, svc.invoked, "the viewer must not be fetched for a rejected invocation")
}

// TestWhoamiSurfacesDialerError pins the seam's failure path: the dialer
// resolves the account for the invocation, so its error (unknown account,
// missing credential) surfaces exactly like every other leaf's dial error.
func TestWhoamiSurfacesDialerError(t *testing.T) {
	dialErr := errors.New(`no linear account "work"`)
	_, root, out := newAccountEnv(t, realStrategy(t), func(context.Context) (service.ViewerService, error) {
		return nil, dialErr
	})

	_, err := execute(t, root, out, "account", "whoami")
	require.ErrorIs(t, err, dialErr)
}

// TestWhoamiSurfacesViewerError pins the GetViewer failure path: the leaf
// returns it as-is rather than wrapping, the same discipline as the other
// dialer-fed leaves.
func TestWhoamiSurfacesViewerError(t *testing.T) {
	svc := &fakeViewerService{err: errors.New("linear API reported no viewer")}
	_, root, out := newAccountEnv(t, realStrategy(t), viewerDialerFor(svc))

	_, err := execute(t, root, out, "account", "whoami")
	require.ErrorContains(t, err, "linear API reported no viewer")
}

// decodeJSONMap decodes stdout as a single JSON object — whoami is a
// one-row command, so an array here is a bug worth failing on.
func decodeJSONMap(t *testing.T, stdout string) (map[string]any, bool) {
	t.Helper()
	var m map[string]any
	err := json.Unmarshal([]byte(stdout), &m)
	return m, err == nil
}
