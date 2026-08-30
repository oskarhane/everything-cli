package event

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	calendar "google.golang.org/api/calendar/v3"
)

// runRespond builds and executes one response leaf against svc.
func runRespond(t *testing.T, svc *fakeEventService, verb string, args ...string) string {
	t.Helper()
	cmd := newRespondCmd(newTestConfig("json"), fakeNewSvc(svc), verb)
	return runCmd(t, cmd, args...)
}

func TestRespondInstancePatchesOnlyThatOccurrence(t *testing.T) {
	verbs := map[string]string{
		"accept":    "accepted",
		"decline":   "declined",
		"tentative": "tentative",
	}
	for verb, status := range verbs {
		t.Run(verb, func(t *testing.T) {
			svc := &fakeEventService{events: seedSeries()}
			runRespond(t, svc, verb, instanceEventID)

			// One fetch of the given id, one patch of the SAME id.
			require.Equal(t, []string{instanceEventID}, svc.getCalls)
			require.Len(t, svc.patches, 1)
			p := svc.patches[0]
			require.Equal(t, instanceEventID, p.eventID, "an instance id responds to that one occurrence only")
			require.Equal(t, "primary", p.calendarID)
			require.Equal(t, "none", p.sendUpdates, "responding sends no update emails")

			// The patch echoes the FULL attendees array, with the self entry's
			// response status flipped and everyone else untouched.
			require.Len(t, p.event.Attendees, 2)
			self := findSelf(t, p.event.Attendees)
			require.Equal(t, status, self.ResponseStatus)
			for _, a := range p.event.Attendees {
				if !a.Self {
					require.Equal(t, "accepted", a.ResponseStatus)
				}
			}
		})
	}
}

func TestRespondAllWithInstanceIDPatchesMaster(t *testing.T) {
	svc := &fakeEventService{events: seedSeries()}
	runRespond(t, svc, "decline", instanceEventID, "--all")

	// The instance is fetched first, then its master is refetched so the
	// patch echoes the master's attendee list.
	require.Equal(t, []string{instanceEventID, masterEventID}, svc.getCalls)
	require.Len(t, svc.patches, 1)
	require.Equal(t, masterEventID, svc.patches[0].eventID, "--all patches the derived master id")
	require.Len(t, svc.patches[0].event.Attendees, 2)
}

func TestRespondThisOnlyFalseWithInstancePatchesMaster(t *testing.T) {
	svc := &fakeEventService{events: seedSeries()}
	runRespond(t, svc, "accept", instanceEventID, "--this-only=false")

	require.Equal(t, []string{instanceEventID, masterEventID}, svc.getCalls)
	require.Len(t, svc.patches, 1)
	require.Equal(t, masterEventID, svc.patches[0].eventID)
}

func TestRespondMasterIDPatchesTheSeries(t *testing.T) {
	svc := &fakeEventService{events: seedSeries()}
	runRespond(t, svc, "tentative", masterEventID)

	require.Equal(t, []string{masterEventID}, svc.getCalls)
	require.Len(t, svc.patches, 1)
	require.Equal(t, masterEventID, svc.patches[0].eventID)
	require.Equal(t, "tentative", findSelf(t, svc.patches[0].event.Attendees).ResponseStatus)
}

func TestRespondAllWithMasterIDStaysOnMaster(t *testing.T) {
	svc := &fakeEventService{events: seedSeries()}
	runRespond(t, svc, "accept", masterEventID, "--all")

	require.Equal(t, []string{masterEventID}, svc.getCalls, "--all on a master id fetches nothing extra")
	require.Len(t, svc.patches, 1)
	require.Equal(t, masterEventID, svc.patches[0].eventID)
}

func TestRespondNotAnAttendee(t *testing.T) {
	series := seedSeries()
	for _, ev := range series {
		attendees := make([]*calendar.EventAttendee, 0, len(ev.Attendees))
		for _, a := range ev.Attendees {
			if a.Self {
				continue
			}
			attendees = append(attendees, a)
		}
		ev.Attendees = attendees
	}
	svc := &fakeEventService{events: series}
	_, err := runCmdErr(t, newRespondCmd(newTestConfig("json"), fakeNewSvc(svc), "decline"), masterEventID)

	require.Contains(t, err.Error(), "you are not an attendee of this event")
	require.Empty(t, svc.patches)
}

func TestRespondPropagatesAPIError(t *testing.T) {
	svc := &fakeEventService{events: seedSeries(), patchErr: errors.New("googleapi: Error 403")}
	_, err := runCmdErr(t, newRespondCmd(newTestConfig("json"), fakeNewSvc(svc), "decline"), instanceEventID)

	require.Contains(t, err.Error(), "googleapi: Error 403")
	require.Len(t, svc.patches, 1, "the patch was attempted and its error surfaced")
}

func TestRespondRequiresExactlyOneArg(t *testing.T) {
	svc := &fakeEventService{events: seedSeries()}
	_, err := runCmdErr(t, newRespondCmd(newTestConfig("json"), fakeNewSvc(svc), "accept"))

	require.Contains(t, err.Error(), "accepts 1 arg")
}
