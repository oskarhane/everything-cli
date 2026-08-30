// Package service is the seam between the calendar command leaves and the
// Google Calendar API. Leaves depend on the CalendarService interface so
// tests can run hermetically against a fake; only this package talks to the
// real API.
//
// The interface covers only what the calendar CRUD and acl leaves need.
// Later subtrees (event, freebusy) add their own interfaces in their own
// files here rather than growing this one.
package service

import (
	"context"
	"fmt"

	"golang.org/x/oauth2"
	calendar "google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// CalendarService is the Calendar API surface used by the calendarlist and
// acl leaves. Thin wrappers over CalendarList, Calendars, and Acl, so fakes
// model resources, not call objects.
//
// colorId lives on the calendar list entry, not on the Calendar resource,
// so color reads and writes go through the CalendarList methods.
type CalendarService interface {
	ListCalendarList(ctx context.Context) ([]*calendar.CalendarListEntry, error)
	GetCalendarList(ctx context.Context, calendarID string) (*calendar.CalendarListEntry, error)
	PatchCalendarList(ctx context.Context, calendarID string, entry *calendar.CalendarListEntry) (*calendar.CalendarListEntry, error)
	GetCalendar(ctx context.Context, calendarID string) (*calendar.Calendar, error)
	InsertCalendar(ctx context.Context, cal *calendar.Calendar) (*calendar.Calendar, error)
	PatchCalendar(ctx context.Context, calendarID string, cal *calendar.Calendar) (*calendar.Calendar, error)
	DeleteCalendar(ctx context.Context, calendarID string) error
	ListAcl(ctx context.Context, calendarID string) ([]*calendar.AclRule, error)
	InsertAcl(ctx context.Context, calendarID string, rule *calendar.AclRule) (*calendar.AclRule, error)
	DeleteAcl(ctx context.Context, calendarID string, ruleID string) error
}

// New returns a CalendarService bound to ts.
func New(ctx context.Context, ts oauth2.TokenSource) (CalendarService, error) {
	svc, err := calendar.NewService(ctx, option.WithTokenSource(ts))
	if err != nil {
		return nil, fmt.Errorf("creating calendar service: %w", err)
	}
	return &realCalendarService{svc: svc}, nil
}

// realCalendarService adapts *calendar.Service to CalendarService.
type realCalendarService struct {
	svc *calendar.Service
}

func (s *realCalendarService) ListCalendarList(ctx context.Context) ([]*calendar.CalendarListEntry, error) {
	resp, err := s.svc.CalendarList.List().Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("listing calendars: %w", err)
	}
	return resp.Items, nil
}

func (s *realCalendarService) GetCalendarList(ctx context.Context, calendarID string) (*calendar.CalendarListEntry, error) {
	entry, err := s.svc.CalendarList.Get(calendarID).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("getting calendar list entry %s: %w", calendarID, err)
	}
	return entry, nil
}

func (s *realCalendarService) PatchCalendarList(ctx context.Context, calendarID string, entry *calendar.CalendarListEntry) (*calendar.CalendarListEntry, error) {
	patched, err := s.svc.CalendarList.Patch(calendarID, entry).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("patching calendar list entry %s: %w", calendarID, err)
	}
	return patched, nil
}

func (s *realCalendarService) GetCalendar(ctx context.Context, calendarID string) (*calendar.Calendar, error) {
	cal, err := s.svc.Calendars.Get(calendarID).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("getting calendar %s: %w", calendarID, err)
	}
	return cal, nil
}

func (s *realCalendarService) InsertCalendar(ctx context.Context, cal *calendar.Calendar) (*calendar.Calendar, error) {
	created, err := s.svc.Calendars.Insert(cal).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("creating calendar: %w", err)
	}
	return created, nil
}

func (s *realCalendarService) PatchCalendar(ctx context.Context, calendarID string, cal *calendar.Calendar) (*calendar.Calendar, error) {
	patched, err := s.svc.Calendars.Patch(calendarID, cal).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("patching calendar %s: %w", calendarID, err)
	}
	return patched, nil
}

func (s *realCalendarService) DeleteCalendar(ctx context.Context, calendarID string) error {
	if err := s.svc.Calendars.Delete(calendarID).Context(ctx).Do(); err != nil {
		return fmt.Errorf("deleting calendar %s: %w", calendarID, err)
	}
	return nil
}

func (s *realCalendarService) ListAcl(ctx context.Context, calendarID string) ([]*calendar.AclRule, error) {
	resp, err := s.svc.Acl.List(calendarID).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("listing acl rules for calendar %s: %w", calendarID, err)
	}
	return resp.Items, nil
}

func (s *realCalendarService) InsertAcl(ctx context.Context, calendarID string, rule *calendar.AclRule) (*calendar.AclRule, error) {
	inserted, err := s.svc.Acl.Insert(calendarID, rule).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("adding acl rule to calendar %s: %w", calendarID, err)
	}
	return inserted, nil
}

func (s *realCalendarService) DeleteAcl(ctx context.Context, calendarID string, ruleID string) error {
	if err := s.svc.Acl.Delete(calendarID, ruleID).Context(ctx).Do(); err != nil {
		return fmt.Errorf("deleting acl rule %s from calendar %s: %w", ruleID, calendarID, err)
	}
	return nil
}
