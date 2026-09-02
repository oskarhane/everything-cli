package acl

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/spf13/cobra"

	calendar "google.golang.org/api/calendar/v3"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
	"github.com/oskarhane/everything-cli/internal/providers/google/calendar/service"
	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

// TestMain neutralizes format auto-detection so the host's harness env and
// TTY cannot flip output expectations.
func TestMain(m *testing.M) {
	output.IsAgent = func() bool { return false }
	output.StdoutIsTerminal = func() bool { return false }
	os.Exit(m.Run())
}

// fakeService is the hermetic service.CalendarService double: it serves
// seeded ACL rules and records writes for assertions. The calendar methods
// are stubs this subtree never exercises.
type fakeService struct {
	rules         []*calendar.AclRule // served by ListAcl
	listErr       error
	inserted      *calendar.AclRule // last InsertAcl request
	insertID      string            // calendar id of the last InsertAcl
	insertErr     error
	deleteCalled  bool
	deletedCalID  string
	deletedRuleID string
	deleteErr     error
}

func (f *fakeService) ListAcl(context.Context, string) ([]*calendar.AclRule, error) {
	return f.rules, f.listErr
}

func (f *fakeService) InsertAcl(_ context.Context, calendarID string, rule *calendar.AclRule) (*calendar.AclRule, error) {
	if f.insertErr != nil {
		return nil, f.insertErr
	}
	f.insertID = calendarID
	f.inserted = rule
	return &calendar.AclRule{Id: "user:" + rule.Scope.Value, Scope: rule.Scope, Role: rule.Role}, nil
}

func (f *fakeService) DeleteAcl(_ context.Context, calendarID string, ruleID string) error {
	f.deleteCalled = true
	f.deletedCalID = calendarID
	f.deletedRuleID = ruleID
	return f.deleteErr
}

func (f *fakeService) ListCalendarList(context.Context) ([]*calendar.CalendarListEntry, error) {
	return nil, fmt.Errorf("not implemented by the acl fake")
}

func (f *fakeService) GetCalendarList(context.Context, string) (*calendar.CalendarListEntry, error) {
	return nil, fmt.Errorf("not implemented by the acl fake")
}

func (f *fakeService) PatchCalendarList(context.Context, string, *calendar.CalendarListEntry) (*calendar.CalendarListEntry, error) {
	return nil, fmt.Errorf("not implemented by the acl fake")
}

func (f *fakeService) GetCalendar(context.Context, string) (*calendar.Calendar, error) {
	return nil, fmt.Errorf("not implemented by the acl fake")
}

func (f *fakeService) InsertCalendar(context.Context, *calendar.Calendar) (*calendar.Calendar, error) {
	return nil, fmt.Errorf("not implemented by the acl fake")
}

func (f *fakeService) PatchCalendar(context.Context, string, *calendar.Calendar) (*calendar.Calendar, error) {
	return nil, fmt.Errorf("not implemented by the acl fake")
}

func (f *fakeService) DeleteCalendar(context.Context, string) error {
	return fmt.Errorf("not implemented by the acl fake")
}

// fakeNewSvc returns a service.Dialer[service.CalendarService] handing out svc, so leaves run hermeti-
// cally with no network and no real account store.
func fakeNewSvc(svc *fakeService) service.Dialer[service.CalendarService] {
	return func(context.Context) (service.CalendarService, error) { return svc, nil }
}

// newLeafCmd builds a leaf against a fake service, ready to execute.
func newLeafCmd(build func(*app.Config, service.Dialer[service.CalendarService]) *cobra.Command, svc *fakeService, format string) *cobra.Command {
	return build(cmdtest.NewTestConfig(format), fakeNewSvc(svc))
}

// seedRules returns a small realistic ACL rule set.
func seedRules() []*calendar.AclRule {
	return []*calendar.AclRule{
		{Id: "user:colleague@example.com", Scope: &calendar.AclRuleScope{Type: "user", Value: "colleague@example.com"}, Role: "reader"},
		{Scope: &calendar.AclRuleScope{Type: "default"}, Role: "none"},
	}
}
