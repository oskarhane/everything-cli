package issue

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
)

// stateLookupRequired reports whether value needs a name→UUID lookup. Empty
// and UUID-shaped values pass through, so callers can skip dialing the state
// service entirely for them.
func stateLookupRequired(value string) bool {
	if value == "" {
		return false
	}
	_, err := uuid.Parse(value)
	return err != nil
}

// resolveStateIDForTeam is the leaf entry point: it dials the state service
// only when value actually needs a name lookup, so UUID and empty --state
// values never dial at all.
func resolveStateIDForTeam(ctx context.Context, newState service.Dialer[service.StateService], teamID, value string) (string, error) {
	if !stateLookupRequired(value) {
		return value, nil
	}
	states, err := newState(ctx)
	if err != nil {
		return "", err
	}
	return resolveStateID(ctx, states, teamID, value)
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
	all, err := states.ListStates(ctx, teamID)
	if err != nil {
		return "", err
	}
	var matches []service.State
	for _, s := range all {
		if strings.EqualFold(s.Name, value) {
			matches = append(matches, s)
		}
	}
	if len(matches) != 1 {
		return "", fmt.Errorf("unknown state %q; valid states: %s", value, stateNames(all))
	}
	return matches[0].ID, nil
}

// stateNames joins the team's state names for the unknown-state error. It
// falls back to "none" so a team with no states still reads sensibly.
func stateNames(states []service.State) string {
	if len(states) == 0 {
		return "none"
	}
	names := make([]string, len(states))
	for i, s := range states {
		names[i] = s.Name
	}
	return strings.Join(names, ", ")
}
