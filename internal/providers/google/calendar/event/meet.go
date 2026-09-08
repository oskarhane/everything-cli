package event

import (
	"fmt"

	"github.com/google/uuid"

	calendar "google.golang.org/api/calendar/v3"
)

// meetConferenceData returns the conferenceData body --meet attaches: a
// hangoutsMeet createRequest with a fresh request id. The id must be minted
// per call because the API dedupes conference creates on it — a repeated id
// would silently drop the create — and one CLI invocation makes exactly one
// insert or patch, so every call needs its own.
func meetConferenceData() *calendar.ConferenceData {
	return &calendar.ConferenceData{
		CreateRequest: &calendar.CreateConferenceRequest{
			RequestId:             uuid.NewString(),
			ConferenceSolutionKey: &calendar.ConferenceSolutionKey{Type: "hangoutsMeet"},
		},
	}
}

// meetLinkMissing is the post-write guard for --meet: the write asked the
// API for a conference, so an event that comes back without a hangout link
// means the ask did not land, and the caller must fail instead of printing
// a link-less view as if it had.
func meetLinkMissing(meet bool, ev *calendar.Event) error {
	if meet && ev.HangoutLink == "" {
		return fmt.Errorf("--meet was passed but the event carries no Meet link")
	}
	return nil
}
