package service

import (
	"context"
	"fmt"

	calendar "google.golang.org/api/calendar/v3"
)

// EventService is the Calendar events API surface used by the `calendar
// event` leaves. Thin wrappers over Events, so fakes model events, not call
// objects. List options arrive via params structs instead of long positional
// argument lists.
type EventService interface {
	// ListEvents lists events. SingleEvents=true expands recurring masters
	// into instances (requires OrderBy "startTime"); false returns masters,
	// single events, and already-materialized exceptions.
	ListEvents(ctx context.Context, params ListEventsParams) ([]*calendar.Event, error)
	// ListInstances expands one recurring event into its occurrences.
	ListInstances(ctx context.Context, params ListInstancesParams) ([]*calendar.Event, error)
	GetEvent(ctx context.Context, calendarID, eventID string) (*calendar.Event, error)
	InsertEvent(ctx context.Context, calendarID string, ev *calendar.Event, sendUpdates string) (*calendar.Event, error)
	PatchEvent(ctx context.Context, calendarID, eventID string, ev *calendar.Event, sendUpdates string) (*calendar.Event, error)
	DeleteEvent(ctx context.Context, calendarID, eventID string, sendUpdates string) error
	MoveEvent(ctx context.Context, calendarID, eventID, destCalendarID string) (*calendar.Event, error)
}

// ListEventsParams carries the events.list options. TimeMin and TimeMax must
// be RFC3339 with an offset; they bound with overlap semantics (end > timeMin
// AND start < timeMax). MaxResults is the TOTAL cap across all pages
// (pagination stops once reached, per-page size honors the remaining
// budget); 0 or less means no cap.
type ListEventsParams struct {
	CalendarID   string
	SingleEvents bool
	TimeMin      string
	TimeMax      string
	Query        string
	MaxResults   int64
	// OrderBy is only legal with SingleEvents=true ("startTime").
	OrderBy string
}

// ListInstancesParams carries the events.instances options; EventID is the
// recurring (master) event id. MaxResults is the TOTAL cap across all pages
// (pagination stops once reached, per-page size honors the remaining
// budget); 0 or less means no cap.
type ListInstancesParams struct {
	CalendarID string
	EventID    string
	TimeMin    string
	TimeMax    string
	MaxResults int64
}

// ListEvents implements EventService. Empty params are left unset so the
// request never carries e.g. an empty timeMin, which the API rejects.
func (s *realCalendarService) ListEvents(ctx context.Context, p ListEventsParams) ([]*calendar.Event, error) {
	call := s.svc.Events.List(p.CalendarID).SingleEvents(p.SingleEvents)
	if p.TimeMin != "" {
		call = call.TimeMin(p.TimeMin)
	}
	if p.TimeMax != "" {
		call = call.TimeMax(p.TimeMax)
	}
	if p.Query != "" {
		call = call.Q(p.Query)
	}
	if p.OrderBy != "" {
		call = call.OrderBy(p.OrderBy)
	}
	return pageAllBudgeted(p.MaxResults, func(page string, remaining int64) ([]*calendar.Event, string, error) {
		if remaining > 0 {
			// Per-request MaxResults honors the remaining budget: later
			// pages never ask for more than the cap still allows.
			call = call.MaxResults(remaining)
		}
		if page != "" {
			call = call.PageToken(page)
		}
		resp, err := call.Context(ctx).Do()
		if err != nil {
			return nil, "", fmt.Errorf("listing events of calendar %s: %w", p.CalendarID, err)
		}
		return resp.Items, resp.NextPageToken, nil
	})
}

// ListInstances implements EventService. Empty params are left unset.
// MaxResults caps the total like ListEvents.
func (s *realCalendarService) ListInstances(ctx context.Context, p ListInstancesParams) ([]*calendar.Event, error) {
	call := s.svc.Events.Instances(p.CalendarID, p.EventID)
	if p.TimeMin != "" {
		call = call.TimeMin(p.TimeMin)
	}
	if p.TimeMax != "" {
		call = call.TimeMax(p.TimeMax)
	}
	return pageAllBudgeted(p.MaxResults, func(page string, remaining int64) ([]*calendar.Event, string, error) {
		if remaining > 0 {
			call = call.MaxResults(remaining)
		}
		if page != "" {
			call = call.PageToken(page)
		}
		resp, err := call.Context(ctx).Do()
		if err != nil {
			return nil, "", fmt.Errorf("listing instances of event %s: %w", p.EventID, err)
		}
		return resp.Items, resp.NextPageToken, nil
	})
}

// GetEvent implements EventService. It accepts master and instance ids.
func (s *realCalendarService) GetEvent(ctx context.Context, calendarID, eventID string) (*calendar.Event, error) {
	ev, err := s.svc.Events.Get(calendarID, eventID).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("getting event %s: %w", eventID, err)
	}
	return ev, nil
}

// InsertEvent implements EventService.
func (s *realCalendarService) InsertEvent(ctx context.Context, calendarID string, ev *calendar.Event, sendUpdates string) (*calendar.Event, error) {
	call := s.svc.Events.Insert(calendarID, ev)
	if sendUpdates != "" {
		call = call.SendUpdates(sendUpdates)
	}
	created, err := call.Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("creating event: %w", err)
	}
	return created, nil
}

// PatchEvent implements EventService. Patching an instance id modifies only
// that occurrence (creating an exception); the same call on a master id
// modifies the whole series. Array fields overwrite wholesale, so callers
// must echo full arrays (attendees).
func (s *realCalendarService) PatchEvent(ctx context.Context, calendarID, eventID string, ev *calendar.Event, sendUpdates string) (*calendar.Event, error) {
	call := s.svc.Events.Patch(calendarID, eventID, ev)
	if sendUpdates != "" {
		call = call.SendUpdates(sendUpdates)
	}
	patched, err := call.Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("patching event %s: %w", eventID, err)
	}
	return patched, nil
}

// DeleteEvent implements EventService. Deleting an instance id cancels only
// that occurrence; deleting a master id deletes the entire series.
func (s *realCalendarService) DeleteEvent(ctx context.Context, calendarID, eventID string, sendUpdates string) error {
	call := s.svc.Events.Delete(calendarID, eventID)
	if sendUpdates != "" {
		call = call.SendUpdates(sendUpdates)
	}
	if err := call.Context(ctx).Do(); err != nil {
		return fmt.Errorf("deleting event %s: %w", eventID, err)
	}
	return nil
}

// MoveEvent implements EventService.
func (s *realCalendarService) MoveEvent(ctx context.Context, calendarID, eventID, destCalendarID string) (*calendar.Event, error) {
	moved, err := s.svc.Events.Move(calendarID, eventID, destCalendarID).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("moving event %s to calendar %s: %w", eventID, destCalendarID, err)
	}
	return moved, nil
}
