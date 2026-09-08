package event

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

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

// frozenNow anchors every relative window and default in the tests.
var frozenNow = time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)

// freezeNow pins the package clock for the test and restores it after.
func freezeNow(t *testing.T) {
	t.Helper()
	original := nowFunc
	nowFunc = func() time.Time { return frozenNow }
	t.Cleanup(func() { nowFunc = original })
}

// fakeHangoutLink is the Meet link the fake mints on insert/patch responses
// whose event body requests a conference.
const fakeHangoutLink = "https://meet.google.com/fake-conf-link"

// fakeEventService is the hermetic service.EventService double: it serves
// seeded events and records every write for assertions.
type fakeEventService struct {
	items           []*calendar.Event // served by ListEvents and ListInstances
	listParams      []service.ListEventsParams
	listErr         error
	instancesParams []service.ListInstancesParams
	instancesErr    error

	events   map[string]*calendar.Event // served by GetEvent, by id
	getCalls []string
	getErr   error

	insertCalID string
	inserted    *calendar.Event
	insertSend  string
	insertErr   error

	patches  []patchCall
	patchErr error

	deletes   []deleteCall
	deleteErr error

	moveCalls []moveCall
	moveErr   error

	// noHangoutLink suppresses the fake's Meet link on conference-requesting
	// insert/patch responses, simulating the API returning none.
	noHangoutLink bool
}

// patchCall records one PatchEvent request.
type patchCall struct {
	calendarID  string
	eventID     string
	event       *calendar.Event
	sendUpdates string
}

// deleteCall records one DeleteEvent request.
type deleteCall struct {
	calendarID  string
	eventID     string
	sendUpdates string
}

// moveCall records one MoveEvent request.
type moveCall struct {
	calendarID     string
	eventID        string
	destCalendarID string
}

// requestsConference reports whether an event body asks the API to create a
// conference (a non-nil conferenceData.createRequest).
func requestsConference(ev *calendar.Event) bool {
	return ev != nil && ev.ConferenceData != nil && ev.ConferenceData.CreateRequest != nil
}

func (f *fakeEventService) ListEvents(_ context.Context, params service.ListEventsParams) ([]*calendar.Event, error) {
	f.listParams = append(f.listParams, params)
	return f.items, f.listErr
}

func (f *fakeEventService) ListInstances(_ context.Context, params service.ListInstancesParams) ([]*calendar.Event, error) {
	f.instancesParams = append(f.instancesParams, params)
	return f.items, f.instancesErr
}

func (f *fakeEventService) GetEvent(_ context.Context, _, eventID string) (*calendar.Event, error) {
	f.getCalls = append(f.getCalls, eventID)
	if f.getErr != nil {
		return nil, f.getErr
	}
	ev, ok := f.events[eventID]
	if !ok {
		return nil, fmt.Errorf("googleapi: Error 404: event %s not found", eventID)
	}
	return ev, nil
}

func (f *fakeEventService) InsertEvent(_ context.Context, calendarID string, ev *calendar.Event, sendUpdates string) (*calendar.Event, error) {
	if f.insertErr != nil {
		return nil, f.insertErr
	}
	f.insertCalID = calendarID
	f.inserted = ev
	f.insertSend = sendUpdates
	created := *ev
	created.Id = "created123"
	if requestsConference(ev) && !f.noHangoutLink {
		// The fake API mints a Meet link for conference-requesting bodies.
		created.HangoutLink = fakeHangoutLink
	}
	return &created, nil
}

func (f *fakeEventService) PatchEvent(_ context.Context, calendarID, eventID string, ev *calendar.Event, sendUpdates string) (*calendar.Event, error) {
	f.patches = append(f.patches, patchCall{calendarID: calendarID, eventID: eventID, event: ev, sendUpdates: sendUpdates})
	if f.patchErr != nil {
		return nil, f.patchErr
	}
	base, ok := f.events[eventID]
	if !ok {
		base = &calendar.Event{Id: eventID}
	}
	resp := *base
	if ev != nil {
		resp.Attendees = ev.Attendees
		if ev.Summary != "" {
			resp.Summary = ev.Summary
		}
		if ev.Start != nil {
			resp.Start = ev.Start
		}
		if ev.End != nil {
			resp.End = ev.End
		}
		if ev.Location != "" {
			resp.Location = ev.Location
		}
		if ev.Description != "" {
			resp.Description = ev.Description
		}
	}
	// A conference-requesting patch gets a fresh Meet link back — or none at
	// all when suppressed; patches that don't request one leave the base's
	// link (seeded or empty) untouched.
	if requestsConference(ev) {
		if f.noHangoutLink {
			resp.HangoutLink = ""
		} else {
			resp.HangoutLink = fakeHangoutLink
		}
	}
	return &resp, nil
}

func (f *fakeEventService) DeleteEvent(_ context.Context, calendarID, eventID string, sendUpdates string) error {
	f.deletes = append(f.deletes, deleteCall{calendarID: calendarID, eventID: eventID, sendUpdates: sendUpdates})
	return f.deleteErr
}

func (f *fakeEventService) MoveEvent(_ context.Context, calendarID, eventID, destCalendarID string) (*calendar.Event, error) {
	f.moveCalls = append(f.moveCalls, moveCall{calendarID: calendarID, eventID: eventID, destCalendarID: destCalendarID})
	if f.moveErr != nil {
		return nil, f.moveErr
	}
	base, ok := f.events[eventID]
	if !ok {
		base = &calendar.Event{Id: eventID}
	}
	moved := *base
	return &moved, nil
}

// fakeNewSvc returns a service.Dialer[service.EventService] handing out svc, so leaves run hermeti-
// cally with no network and no real account store.
func fakeNewSvc(svc *fakeEventService) service.Dialer[service.EventService] {
	return func(context.Context) (service.EventService, error) { return svc, nil }
}

// newLeafCmd builds a leaf against a fake service, ready to execute.
func newLeafCmd(build func(*app.Config, service.Dialer[service.EventService]) *cobra.Command, svc *fakeEventService, format string) *cobra.Command {
	return build(cmdtest.NewTestConfig(format), fakeNewSvc(svc))
}

// Ids of the seeded recurring series (distinct from the masterID helper): a weekly master and one instance.
const (
	masterEventID   = "kq3abc123"
	instanceEventID = "kq3abc123_20260901T090000Z"
)

// seedSeries returns a recurring master and one of its instances, each with
// an organizer attendee and a self (acting-account) attendee. Fresh objects
// per call so a test mutating one cannot bleed into another.
func seedSeries() map[string]*calendar.Event {
	attendees := func() []*calendar.EventAttendee {
		return []*calendar.EventAttendee{
			{Email: "organizer@example.com", ResponseStatus: "accepted", Organizer: true},
			{Email: "me@example.com", ResponseStatus: "needsAction", Self: true},
		}
	}
	return map[string]*calendar.Event{
		masterEventID: {
			Id:         masterEventID,
			Summary:    "Weekly standup",
			Recurrence: []string{"RRULE:FREQ=WEEKLY;COUNT=10"},
			Start:      &calendar.EventDateTime{DateTime: "2026-09-01T09:00:00Z", TimeZone: "UTC"},
			End:        &calendar.EventDateTime{DateTime: "2026-09-01T09:30:00Z", TimeZone: "UTC"},
			Attendees:  attendees(),
		},
		instanceEventID: {
			Id:               instanceEventID,
			Summary:          "Weekly standup",
			RecurringEventId: masterEventID,
			Start:            &calendar.EventDateTime{DateTime: "2026-09-01T09:00:00Z", TimeZone: "UTC"},
			End:              &calendar.EventDateTime{DateTime: "2026-09-01T09:30:00Z", TimeZone: "UTC"},
			Attendees:        attendees(),
		},
	}
}

// seedListEvents returns one single event, one recurring master, and one
// expanded instance, covering every row shape `event list` can emit.
func seedListEvents() []*calendar.Event {
	series := seedSeries()
	return []*calendar.Event{
		{Id: "sing111", Summary: "One-off sync", Start: &calendar.EventDateTime{DateTime: "2026-09-02T10:00:00Z"}, End: &calendar.EventDateTime{DateTime: "2026-09-02T10:30:00Z"}},
		series[masterEventID],
		series[instanceEventID],
	}
}

// cancelledEventID is the id of the seeded cancelled event.
const cancelledEventID = "cxl999"

// appendCancelled returns the given list plus a cancelled instance (status
// "cancelled", the shape --show-deleted is meant to surface), so list tests
// can observe the status field.
func appendCancelled(events []*calendar.Event) []*calendar.Event {
	return append(events, &calendar.Event{
		Id:      cancelledEventID,
		Summary: "Cancelled sync",
		Status:  "cancelled",
		Start:   &calendar.EventDateTime{DateTime: "2026-09-02T11:00:00Z", TimeZone: "UTC"},
		End:     &calendar.EventDateTime{DateTime: "2026-09-02T11:30:00Z", TimeZone: "UTC"},
	})
}

// seedAllDayEvent returns an all-day event for update date handling.
func seedAllDayEvent() map[string]*calendar.Event {
	return map[string]*calendar.Event{
		"allday1": {
			Id:      "allday1",
			Summary: "Conference",
			Start:   &calendar.EventDateTime{Date: "2026-10-01"},
			End:     &calendar.EventDateTime{Date: "2026-10-03"},
		},
	}
}

// findSelf returns the attendee entry marked self, or nil.
func findSelf(t *testing.T, attendees []*calendar.EventAttendee) *calendar.EventAttendee {
	t.Helper()
	for _, a := range attendees {
		if a.Self {
			return a
		}
	}
	t.Fatalf("no self attendee in %+v", attendees)
	return nil
}

// TestFakeEventServiceHangoutLink pins the fake's conference modeling: the
// fake mints a Meet link iff the request body carries a createRequest,
// noHangoutLink suppresses it, and a seeded base link survives patches that
// don't request a conference.
func TestFakeEventServiceHangoutLink(t *testing.T) {
	const seededLink = "https://meet.google.com/seeded-link"
	newEvent := func(withConf bool) *calendar.Event {
		ev := &calendar.Event{Summary: "Sync"}
		if withConf {
			ev.ConferenceData = &calendar.ConferenceData{
				CreateRequest: &calendar.CreateConferenceRequest{
					ConferenceSolutionKey: &calendar.ConferenceSolutionKey{Type: "hangoutsMeet"},
					RequestId:             "req-1",
				},
			}
		}
		return ev
	}
	tests := []struct {
		name      string
		patch     bool
		noHangout bool
		withConf  bool
		seedLink  string // HangoutLink on the seeded base event (patch only)
		want      string
	}{
		{name: "insert requesting a conference mints the link", withConf: true, want: fakeHangoutLink},
		{name: "insert requesting a conference suppressed", withConf: true, noHangout: true},
		{name: "insert without a conference request has no link"},
		{name: "patch requesting a conference mints the link", patch: true, withConf: true, want: fakeHangoutLink},
		{name: "patch requesting a conference suppressed clears a seeded link", patch: true, withConf: true, noHangout: true, seedLink: seededLink},
		{name: "patch without a conference request keeps a seeded link", patch: true, seedLink: seededLink, want: seededLink},
		{name: "patch requesting a conference replaces a seeded link", patch: true, withConf: true, seedLink: seededLink, want: fakeHangoutLink},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeEventService{noHangoutLink: tt.noHangout}
			if tt.patch {
				fake.events = map[string]*calendar.Event{"ev-1": {Id: "ev-1", HangoutLink: tt.seedLink}}
			}
			var (
				got *calendar.Event
				err error
			)
			if tt.patch {
				got, err = fake.PatchEvent(t.Context(), "cal-1", "ev-1", newEvent(tt.withConf), "")
			} else {
				got, err = fake.InsertEvent(t.Context(), "cal-1", newEvent(tt.withConf), "")
			}
			if err != nil {
				t.Fatalf("%s: %v", tt.name, err)
			}
			if got.HangoutLink != tt.want {
				t.Errorf("HangoutLink = %q, want %q", got.HangoutLink, tt.want)
			}
		})
	}
}
