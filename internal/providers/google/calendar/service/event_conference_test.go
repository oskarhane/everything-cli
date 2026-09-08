package service

import (
	"encoding/json"
	"net/http"
	"testing"

	calendar "google.golang.org/api/calendar/v3"
)

// conferenceRequestCapture records what one events.insert or events.patch
// request carried: the conferenceDataVersion query param and whether the JSON
// body holds a conferenceData.createRequest.
type conferenceRequestCapture struct {
	version          string
	hasCreateRequest bool
}

// TestInsertPatchEventConferenceDataVersion pins the seam's inference rule
// against a hermetic fake of the Calendar REST endpoints: insert and patch
// send conferenceDataVersion=1 and a conferenceData.createRequest body iff
// the event body carries ConferenceData — never one without the other, which
// the real API silently ignores or rejects.
func TestInsertPatchEventConferenceDataVersion(t *testing.T) {
	tests := []struct {
		name             string
		patch            bool // false → insert; true → patch
		withConference   bool // seed ConferenceData.CreateRequest on the body
		wantVersion      string
		wantCreateInBody bool
	}{
		{name: "insert with conference data", withConference: true, wantVersion: "1", wantCreateInBody: true},
		{name: "insert without conference data"},
		{name: "patch with conference data", patch: true, withConference: true, wantVersion: "1", wantCreateInBody: true},
		{name: "patch without conference data", patch: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []conferenceRequestCapture
			capture := func(w http.ResponseWriter, r *http.Request) {
				var body struct {
					ConferenceData *struct {
						CreateRequest json.RawMessage `json:"createRequest"`
					} `json:"conferenceData"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Errorf("decoding request body: %v", err)
				}
				got = append(got, conferenceRequestCapture{
					version:          r.URL.Query().Get("conferenceDataVersion"),
					hasCreateRequest: body.ConferenceData != nil && len(body.ConferenceData.CreateRequest) > 0,
				})
				writeJSON(w, calendar.Event{Id: "x"})
			}
			svc := newPagedTestServer(t, map[string]http.HandlerFunc{
				"/calendars/cal-1/events":      capture, // events.insert
				"/calendars/cal-1/events/ev-1": capture, // events.patch
			})

			ev := &calendar.Event{Summary: "Sync"}
			if tt.withConference {
				// Local copy on purpose: service sits below event at this
				// seam, so importing event's meetConferenceData would invert
				// the dependency; the canonical body and its fresh-request-id
				// rule live in calendar/event/meet.go.
				ev.ConferenceData = &calendar.ConferenceData{
					CreateRequest: &calendar.CreateConferenceRequest{
						ConferenceSolutionKey: &calendar.ConferenceSolutionKey{Type: "hangoutsMeet"},
						RequestId:             "req-1",
					},
				}
			}

			var err error
			if tt.patch {
				_, err = svc.PatchEvent(t.Context(), "cal-1", "ev-1", ev, "")
			} else {
				_, err = svc.InsertEvent(t.Context(), "cal-1", ev, "")
			}
			if err != nil {
				t.Fatalf("%s: %v", tt.name, err)
			}

			if len(got) != 1 {
				t.Fatalf("requests captured = %d, want 1", len(got))
			}
			if got[0].version != tt.wantVersion {
				t.Errorf("conferenceDataVersion = %q, want %q", got[0].version, tt.wantVersion)
			}
			if got[0].hasCreateRequest != tt.wantCreateInBody {
				t.Errorf("conferenceData.createRequest in body = %v, want %v", got[0].hasCreateRequest, tt.wantCreateInBody)
			}
		})
	}
}
