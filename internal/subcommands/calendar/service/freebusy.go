package service

import (
	"context"
	"fmt"

	calendar "google.golang.org/api/calendar/v3"
)

// FreeBusyService is the Calendar API surface used by the `calendar
// freebusy` leaf: the freebusy.query call plus the calendar-list lookup the
// leaf needs to resolve its default calendar set. Thin wrappers, so fakes
// model responses, not call objects. Query options arrive via a params
// struct instead of long positional argument lists.
type FreeBusyService interface {
	ListCalendarList(ctx context.Context) ([]*calendar.CalendarListEntry, error)
	// QueryFreeBusy queries the given calendars for busy intervals. The
	// response is expanded as-is; callers flatten the per-calendar busy maps.
	QueryFreeBusy(ctx context.Context, params QueryFreeBusyParams) (*calendar.FreeBusyResponse, error)
}

// QueryFreeBusyParams carries the freebusy.query options. TimeMin and
// TimeMax must be RFC3339 with an offset; TimeMax is exclusive and TimeMin
// inclusive.
type QueryFreeBusyParams struct {
	TimeMin string
	TimeMax string
	// CalendarIDs lists the calendar (or group) ids to query.
	CalendarIDs []string
}

// AsFreeBusyService adapts the CalendarService returned by New into a
// FreeBusyService. The concrete service implements both interfaces; the
// adapter exists so the calendar parent can hand the freebusy leaf a dialer
// without leaking the concrete type.
func AsFreeBusyService(svc CalendarService, err error) (FreeBusyService, error) {
	if err != nil {
		return nil, err
	}
	fb, ok := svc.(FreeBusyService)
	if !ok {
		return nil, fmt.Errorf("calendar service does not implement the freebusy API")
	}
	return fb, nil
}

// QueryFreeBusy implements FreeBusyService. It always sends TimeMin, TimeMax,
// and at least one item: the API rejects an empty request.
func (s *realCalendarService) QueryFreeBusy(ctx context.Context, p QueryFreeBusyParams) (*calendar.FreeBusyResponse, error) {
	items := make([]*calendar.FreeBusyRequestItem, 0, len(p.CalendarIDs))
	for _, id := range p.CalendarIDs {
		items = append(items, &calendar.FreeBusyRequestItem{Id: id})
	}
	resp, err := s.svc.Freebusy.Query(&calendar.FreeBusyRequest{
		TimeMin: p.TimeMin,
		TimeMax: p.TimeMax,
		Items:   items,
	}).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("querying freebusy: %w", err)
	}
	return resp, nil
}
