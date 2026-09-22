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
// the lookup dial entirely — the same short-circuit stateLookupRequired
// gives --state.
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
// offending entry rather than silently dropping it.
func resolveLabelIDs(ctx context.Context, newLabel service.Dialer[service.LabelService], teamID, value string) ([]string, error) {
	entries := splitList(value)
	if len(entries) == 0 {
		return nil, nil
	}
	if !labelLookupRequired(value) {
		return entries, nil
	}
	labels, err := newLabel(ctx)
	if err != nil {
		return nil, err
	}
	all, err := labels.ListTeamLabels(ctx, teamID)
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
			return nil, fmt.Errorf("unknown label %q; valid labels: %s", entry, labelNames(all))
		}
		ids[i] = id
	}
	return ids, nil
}

// labelNames joins the team's label names for the unknown-label error,
// falling back to "none" so a team with no labels still reads sensibly.
func labelNames(labels []service.Label) string {
	if len(labels) == 0 {
		return "none"
	}
	names := make([]string, len(labels))
	for i, l := range labels {
		names[i] = l.Name
	}
	return strings.Join(names, ", ")
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
// number within the team, failing on zero or several matches so an
// ambiguous value never picks the wrong cycle.
func resolveCycleID(ctx context.Context, newCycle service.Dialer[service.CycleService], teamID, value string) (string, error) {
	if value == "" || isUUID(value) {
		return value, nil
	}
	cycles, err := newCycle(ctx)
	if err != nil {
		return "", err
	}
	all, err := cycles.ListCycles(ctx, teamID)
	if err != nil {
		return "", err
	}
	var matches []service.Cycle
	for _, c := range all {
		if strings.EqualFold(c.Name, value) || strconv.Itoa(c.Number) == value {
			matches = append(matches, c)
		}
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("unknown cycle %q", value)
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("ambiguous cycle %q", value)
	}
	return matches[0].ID, nil
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
	all, err := milestones.ListProjectMilestones(ctx, projectID)
	if err != nil {
		return "", err
	}
	var matches []service.Milestone
	for _, m := range all {
		if strings.EqualFold(m.Name, value) {
			matches = append(matches, m)
		}
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("unknown milestone %q", value)
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("ambiguous milestone %q", value)
	}
	return matches[0].ID, nil
}
