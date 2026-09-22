package issue

import (
	"context"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
)

// writeFakes is the hermetic Label/Cycle/Milestone double for the write
// leaves. It serves seeded values and records lookups; calls points at the
// paired fakeService's call log so tests can pin cross-service sequencing
// (e.g. update's GetIssue-before-ListTeamLabels).
type writeFakes struct {
	labels     []service.Label
	cycles     []service.Cycle
	milestones []service.Milestone
	err        error // when set, every call fails

	calls                 *[]string
	listLabelsTeam        string
	listCyclesTeam        string
	listMilestonesProject string
}

func (f *writeFakes) record(name string) {
	if f.calls != nil {
		*f.calls = append(*f.calls, name)
	}
}

func (f *writeFakes) ListTeamLabels(_ context.Context, teamID string) ([]service.Label, error) {
	f.record("ListTeamLabels")
	f.listLabelsTeam = teamID
	if f.err != nil {
		return nil, f.err
	}
	return f.labels, nil
}

func (f *writeFakes) ListCycles(_ context.Context, teamID string) ([]service.Cycle, error) {
	f.record("ListCycles")
	f.listCyclesTeam = teamID
	if f.err != nil {
		return nil, f.err
	}
	return f.cycles, nil
}

func (f *writeFakes) ListProjectMilestones(_ context.Context, projectID string) ([]service.Milestone, error) {
	f.record("ListProjectMilestones")
	f.listMilestonesProject = projectID
	if f.err != nil {
		return nil, f.err
	}
	return f.milestones, nil
}

// fakeNewLabelSvc, fakeNewCycleSvc, and fakeNewMilestoneSvc hand out wf as
// each write-side surface so name resolution runs hermetically.
func fakeNewLabelSvc(wf *writeFakes) service.Dialer[service.LabelService] {
	return func(context.Context) (service.LabelService, error) { return wf, nil }
}

func fakeNewCycleSvc(wf *writeFakes) service.Dialer[service.CycleService] {
	return func(context.Context) (service.CycleService, error) { return wf, nil }
}

func fakeNewMilestoneSvc(wf *writeFakes) service.Dialer[service.MilestoneService] {
	return func(context.Context) (service.MilestoneService, error) { return wf, nil }
}

// writeLeafBuilder matches the create/update constructor shape with every
// write dialer.
type writeLeafBuilder func(*app.Config, service.Dialer[service.IssueService], service.Dialer[service.StateService], service.Dialer[service.LabelService], service.Dialer[service.CycleService], service.Dialer[service.MilestoneService]) *cobra.Command

// newWriteLeafCmd builds a create/update leaf against the fakes with every
// write dialer, reusing newStateLeafCmd for the issue and state sides.
func newWriteLeafCmd(build writeLeafBuilder, svc *fakeService, wf *writeFakes, format string) *cobra.Command {
	wf.calls = &svc.calls
	return newStateLeafCmd(func(cfg *app.Config, newSvc service.Dialer[service.IssueService], newState service.Dialer[service.StateService]) *cobra.Command {
		return build(cfg, newSvc, newState, fakeNewLabelSvc(wf), fakeNewCycleSvc(wf), fakeNewMilestoneSvc(wf))
	}, svc, format)
}

// seedLabels, seedCycles, and seedMilestones return the lookup fixtures the
// write-flag tests resolve against.
func seedLabels() []service.Label {
	return []service.Label{
		{ID: "label_1", Name: "Bug", Color: "#ff0000"},
		{ID: "label_2", Name: "Feature", Color: "#00ff00"},
	}
}

func seedCycles() []service.Cycle {
	return []service.Cycle{
		{ID: "cycle_1", Name: "Sprint 12", Number: 12},
		{ID: "cycle_2", Name: "", Number: 13},
	}
}

func seedMilestones() []service.Milestone {
	return []service.Milestone{
		{ID: "milestone_1", Name: "Beta"},
		{ID: "milestone_2", Name: "GA"},
	}
}

// seedParentIssue returns a second issue so --parent can resolve a human
// identifier to its UUID.
func seedParentIssue() service.Issue {
	return service.Issue{ID: "issue_9", Identifier: "ENG-9", Title: "Parent epic"}
}

func TestResolvePriority(t *testing.T) {
	cases := []struct {
		value string
		want  int
	}{
		{"", 0},
		{"urgent", 1},
		{"High", 2},
		{"MEDIUM", 3},
		{"low", 4},
		{"none", 0},
		{"0", 0},
		{"1", 1},
		{"4", 4},
	}
	for _, tc := range cases {
		got, err := resolvePriority(tc.value)
		require.NoError(t, err, tc.value)
		require.Equal(t, tc.want, got, tc.value)
	}
}

func TestResolvePriorityRejectsOutOfRange(t *testing.T) {
	for _, value := range []string{"5", "-1", "soon"} {
		_, err := resolvePriority(value)
		require.ErrorContains(t, err, value)
	}
}

func TestResolveLabelIDsShortCircuitsUUIDs(t *testing.T) {
	// A nil dialer proves the all-UUID path never dials.
	ids, err := resolveLabelIDs(context.Background(), nil, "team_1", stateUUID+", "+stateUUID)
	require.NoError(t, err)
	require.Equal(t, []string{stateUUID, stateUUID}, ids)
}

func TestResolveLabelIDsReportsTheUnknownName(t *testing.T) {
	wf := &writeFakes{labels: seedLabels()}
	_, err := resolveLabelIDs(context.Background(), fakeNewLabelSvc(wf), "team_1", "Bug, Nope")
	require.ErrorContains(t, err, `unknown label "Nope"`)
	require.Equal(t, "team_1", wf.listLabelsTeam)
}
