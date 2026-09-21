package issue

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
)

// seedStates returns one team's workflow states in board order. IDs are
// stable so tests can assert the resolved UUID.
func seedStates() []service.State {
	return []service.State{
		{ID: "state_1", Name: "Backlog", Type: "backlog", Position: 1},
		{ID: "state_2", Name: "In Progress", Type: "started", Position: 2},
		{ID: "state_3", Name: "Done", Type: "completed", Position: 3},
	}
}

const stateUUID = "8b9c0d1e-1234-5678-9abc-def012345678"

func TestResolveStateIDPassesThroughWithoutLookup(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "empty", value: ""},
		{name: "uuid", value: stateUUID},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeService{states: seedStates()}

			got, err := resolveStateID(context.Background(), svc, "team_1", tt.value)

			require.NoError(t, err)
			require.Equal(t, tt.value, got)
			require.Zero(t, svc.listStatesCalls, "UUID and empty values must not hit ListStates")
		})
	}
}

func TestResolveStateIDMatchesNameCaseInsensitively(t *testing.T) {
	svc := &fakeService{states: seedStates()}

	got, err := resolveStateID(context.Background(), svc, "team_1", "in progress")

	require.NoError(t, err)
	require.Equal(t, "state_2", got)
	require.Equal(t, "team_1", svc.listStatesTeam)
}

func TestResolveStateIDUnknownName(t *testing.T) {
	svc := &fakeService{states: seedStates()}

	_, err := resolveStateID(context.Background(), svc, "team_1", "In Progres")

	require.ErrorContains(t, err, `unknown state "In Progres"`)
	require.ErrorContains(t, err, "Backlog, In Progress, Done")
}

// TestResolveStateIDAmbiguousName pins that two case-insensitive matches
// fail rather than resolving to an arbitrary one.
func TestResolveStateIDAmbiguousName(t *testing.T) {
	svc := &fakeService{states: []service.State{
		{ID: "state_1", Name: "Done", Position: 1},
		{ID: "state_2", Name: "done", Position: 2},
	}}

	_, err := resolveStateID(context.Background(), svc, "team_1", "DONE")

	require.ErrorContains(t, err, `ambiguous state "DONE"`)
	require.ErrorContains(t, err, "Done, done")
}

func TestResolveStateIDListError(t *testing.T) {
	sentinel := errors.New("states unavailable")
	svc := &fakeService{err: sentinel}

	_, err := resolveStateID(context.Background(), svc, "team_1", "Todo")

	require.ErrorIs(t, err, sentinel)
}

// TestResolveStateIDForTeamSkipsDialForUUID pins that the leaf entry point
// does not even dial the state service for a UUID value.
func TestResolveStateIDForTeamSkipsDialForUUID(t *testing.T) {
	dialed := false
	newState := func(context.Context) (service.StateService, error) {
		dialed = true
		return &fakeService{states: seedStates()}, nil
	}

	got, err := resolveStateIDForTeam(context.Background(), newState, "team_1", stateUUID)

	require.NoError(t, err)
	require.Equal(t, stateUUID, got)
	require.False(t, dialed)
}
