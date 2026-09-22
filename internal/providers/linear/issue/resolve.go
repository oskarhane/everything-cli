package issue

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
)

// isUUID reports whether value is UUID-shaped, so already-resolved IDs skip
// the lookup dial entirely.
func isUUID(value string) bool {
	_, err := uuid.Parse(value)
	return err == nil
}

// splitList splits a comma-separated flag value into trimmed entries,
// dropping empties so "--labels a, b" and "--labels a,b" behave alike.
func splitList(value string) []string {
	var entries []string
	for _, entry := range strings.Split(value, ",") {
		if entry = strings.TrimSpace(entry); entry != "" {
			entries = append(entries, entry)
		}
	}
	return entries
}

// joinNames joins item names for an unknown/ambiguous error, falling back
// to "none" so an empty list still reads sensibly.
func joinNames[T any](items []T, name func(T) string) string {
	if len(items) == 0 {
		return "none"
	}
	names := make([]string, len(items))
	for i, item := range items {
		names[i] = name(item)
	}
	return strings.Join(names, ", ")
}

// resolveByName is the shared list→match→resolve engine behind the --state,
// --cycle, and --milestone name lookups: it lists candidates, keeps those
// matching value, and fails on zero or several matches so an ambiguous
// value never picks the wrong one. `what` is the error noun ("state", ...).
func resolveByName[T any](ctx context.Context, list func(context.Context) ([]T, error), value, what string, match func(T, string) bool, name func(T) string, id func(T) string) (string, error) {
	all, err := list(ctx)
	if err != nil {
		return "", err
	}
	var matches []T
	for _, item := range all {
		if match(item, value) {
			matches = append(matches, item)
		}
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("unknown %s %q; valid %ss: %s", what, value, what, joinNames(all, name))
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("ambiguous %s %q; valid %ss: %s", what, value, what, joinNames(all, name))
	}
	return id(matches[0]), nil
}

// resolveParentID turns a --parent value into an issue UUID. Empty and
// UUID-shaped values pass through untouched; a human identifier (ENG-123)
// resolves through GetIssue, which accepts identifiers natively.
func resolveParentID(ctx context.Context, svc service.IssueService, value string) (string, error) {
	if value == "" || isUUID(value) {
		return value, nil
	}
	parent, err := svc.GetIssue(ctx, value)
	if err != nil {
		return "", err
	}
	return parent.ID, nil
}

// labelLookupRequired reports whether a --labels value holds any entry that
// needs a name→UUID lookup; all-UUID (or empty) values never dial.
func labelLookupRequired(value string) bool {
	for _, entry := range splitList(value) {
		if !isUUID(entry) {
			return true
		}
	}
	return false
}

// resolveLabelIDs turns a comma-separated --labels value into label UUIDs.
// UUID entries pass through in place; names resolve case-insensitively
// against the team's labels, and an unknown name errors naming the
// offending entry rather than silently dropping it. teamID is lazy: it is
// only called once an entry actually needs a lookup.
func resolveLabelIDs(ctx context.Context, newLabel service.Dialer[service.LabelService], teamID func() (string, error), value string) ([]string, error) {
	entries := splitList(value)
	if len(entries) == 0 {
		return nil, nil
	}
	if !labelLookupRequired(value) {
		return entries, nil
	}
	team, err := teamID()
	if err != nil {
		return nil, err
	}
	labels, err := newLabel(ctx)
	if err != nil {
		return nil, err
	}
	all, err := labels.ListTeamLabels(ctx, team)
	if err != nil {
		return nil, err
	}
	byName := make(map[string]string, len(all))
	for _, l := range all {
		byName[strings.ToLower(l.Name)] = l.ID
	}
	ids := make([]string, len(entries))
	for i, entry := range entries {
		if isUUID(entry) {
			ids[i] = entry
			continue
		}
		id, ok := byName[strings.ToLower(entry)]
		if !ok {
			return nil, fmt.Errorf("unknown label %q; valid labels: %s", entry, joinNames(all, func(l service.Label) string { return l.Name }))
		}
		ids[i] = id
	}
	return ids, nil
}

// resolvePriority maps a --priority value to Linear's scale: urgent, high,
// medium, low, none → 1, 2, 3, 4, 0. Digits 0-4 pass through for scripts
// that already think in Linear's numbers. Empty means "not set".
func resolvePriority(value string) (int, error) {
	switch strings.ToLower(value) {
	case "":
		return 0, nil
	case "urgent":
		return 1, nil
	case "high":
		return 2, nil
	case "medium":
		return 3, nil
	case "low":
		return 4, nil
	case "none":
		return 0, nil
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 0 || n > 4 {
		return 0, fmt.Errorf("invalid priority %q: use urgent, high, medium, low, none, or 0-4", value)
	}
	return n, nil
}

// resolveCycleID turns a --cycle value into a cycle UUID. Empty and
// UUID-shaped values pass through; any other value matches a cycle name or
// number within the team. teamID is lazy: it is only called when the value
// needs a lookup.
func resolveCycleID(ctx context.Context, newCycle service.Dialer[service.CycleService], teamID func() (string, error), value string) (string, error) {
	if value == "" || isUUID(value) {
		return value, nil
	}
	team, err := teamID()
	if err != nil {
		return "", err
	}
	cycles, err := newCycle(ctx)
	if err != nil {
		return "", err
	}
	return resolveByName(ctx, func(ctx context.Context) ([]service.Cycle, error) {
		return cycles.ListCycles(ctx, team)
	}, value, "cycle",
		func(c service.Cycle, v string) bool {
			return strings.EqualFold(c.Name, v) || strconv.Itoa(c.Number) == v
		},
		func(c service.Cycle) string { return c.Name },
		func(c service.Cycle) string { return c.ID })
}

// resolveMilestoneID turns a --milestone value into a milestone UUID.
// Empty and UUID-shaped values pass through; a name can only resolve within
// one project, so callers without a project ID fail before dialing.
func resolveMilestoneID(ctx context.Context, newMilestone service.Dialer[service.MilestoneService], projectID, value string) (string, error) {
	if value == "" || isUUID(value) {
		return value, nil
	}
	if projectID == "" {
		return "", fmt.Errorf("cannot resolve --milestone %q by name without a project: pass --project", value)
	}
	milestones, err := newMilestone(ctx)
	if err != nil {
		return "", err
	}
	return resolveByName(ctx, func(ctx context.Context) ([]service.NamedRef, error) {
		return milestones.ListProjectMilestones(ctx, projectID)
	}, value, "milestone",
		func(m service.NamedRef, v string) bool { return strings.EqualFold(m.Name, v) },
		func(m service.NamedRef) string { return m.Name },
		func(m service.NamedRef) string { return m.ID })
}
