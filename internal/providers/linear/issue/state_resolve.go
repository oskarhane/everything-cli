package issue

import (
	"context"
	"fmt"
	"strings"

	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
)

// stateLookupRequired reports whether value needs a name→UUID lookup. Empty
// and UUID-shaped values pass through, so callers can skip dialing the state
// service entirely for them.
func stateLookupRequired(value string) bool {
	return value != "" && !isUUID(value)
}

// resolveStateIDForTeam is the leaf entry point: it resolves the team lazily
// and dials the state service only when value actually needs a name lookup,
// so UUID and empty --state values neither fetch the team nor dial.
func resolveStateIDForTeam(ctx context.Context, newState service.Dialer[service.StateService], teamID func() (string, error), value string) (string, error) {
	if !stateLookupRequired(value) {
		return value, nil
	}
	team, err := teamID()
	if err != nil {
		return "", err
	}
	states, err := newState(ctx)
	if err != nil {
		return "", err
	}
	return resolveStateID(ctx, states, team, value)
}

// resolveStateID turns a --state value into a workflow-state UUID. Empty and
// UUID-shaped values are returned unchanged without touching the API; any
// other value is matched case-insensitively against the team's state names.
// A name that matches zero or several states fails rather than picking one,
// since the state the caller meant would be ambiguous.
func resolveStateID(ctx context.Context, states service.StateService, teamID, value string) (string, error) {
	if !stateLookupRequired(value) {
		return value, nil
	}
	if states == nil {
		return "", fmt.Errorf("state %q: no state service available", value)
	}
	return resolveByName(ctx, func(ctx context.Context) ([]service.State, error) {
		return states.ListStates(ctx, teamID)
	}, value, "state",
		func(s service.State, v string) bool { return strings.EqualFold(s.Name, v) },
		func(s service.State) string { return s.Name },
		func(s service.State) string { return s.ID })
}
