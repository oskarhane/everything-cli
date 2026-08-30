package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/oauth2"
	calendar "google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// newPagedTestServer builds a hermetic fake of the Calendar REST endpoints
// listed in pages (path → fetch handler) and returns a realCalendarService
// pointed at it. Each handler sees pageToken; responses are plain JSON
// fragments served verbatim, so the tests below spell out the raw two-page
// shape (items + nextPageToken on the first page, "" on the second).
func newPagedTestServer(t *testing.T, handlers map[string]http.HandlerFunc) *realCalendarService {
	t.Helper()
	mux := http.NewServeMux()
	for path, h := range handlers {
		mux.HandleFunc(path, h)
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	svc, err := calendar.NewService(t.Context(),
		option.WithEndpoint(srv.URL),
		option.WithTokenSource(oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test-token"})),
	)
	if err != nil {
		t.Fatalf("creating calendar service: %v", err)
	}
	return &realCalendarService{svc: svc}
}

// writeJSON serves v as a 200 JSON response.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func TestListCalendarListFollowsAllPages(t *testing.T) {
	svc := newPagedTestServer(t, map[string]http.HandlerFunc{
		"/users/me/calendarList": func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Query().Get("pageToken") {
			case "":
				writeJSON(w, calendar.CalendarList{
					Items:         []*calendar.CalendarListEntry{{Id: "cal-1"}, {Id: "cal-2"}},
					NextPageToken: "tok-2",
				})
			case "tok-2":
				writeJSON(w, calendar.CalendarList{
					Items:         []*calendar.CalendarListEntry{{Id: "cal-3"}},
					NextPageToken: "",
				})
			default:
				http.Error(w, "unexpected pageToken", http.StatusBadRequest)
			}
		},
	})

	entries, err := svc.ListCalendarList(t.Context())
	if err != nil {
		t.Fatalf("ListCalendarList: %v", err)
	}
	var ids []string
	for _, e := range entries {
		ids = append(ids, e.Id)
	}
	// Without paging this returns only cal-1 and cal-2: page one is the last
	// one seen. Asserting all three proves the truncation is gone.
	want := []string{"cal-1", "cal-2", "cal-3"}
	if len(ids) != len(want) {
		t.Fatalf("ListCalendarList ids = %v, want %v", ids, want)
	}
	for i, id := range want {
		if ids[i] != id {
			t.Errorf("entries[%d].Id = %s, want %s", i, ids[i], id)
		}
	}
}

func TestListAclFollowsAllPages(t *testing.T) {
	svc := newPagedTestServer(t, map[string]http.HandlerFunc{
		"/calendars/cal-1/acl": func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Query().Get("pageToken") {
			case "":
				writeJSON(w, calendar.Acl{
					Items: []*calendar.AclRule{
						{Id: "user:one@example.com", Scope: &calendar.AclRuleScope{Type: "user", Value: "one@example.com"}},
					},
					NextPageToken: "tok-2",
				})
			case "tok-2":
				writeJSON(w, calendar.Acl{
					Items: []*calendar.AclRule{
						{Id: "user:two@example.com", Scope: &calendar.AclRuleScope{Type: "user", Value: "two@example.com"}},
					},
					NextPageToken: "",
				})
			default:
				http.Error(w, "unexpected pageToken", http.StatusBadRequest)
			}
		},
	})

	rules, err := svc.ListAcl(t.Context(), "cal-1")
	if err != nil {
		t.Fatalf("ListAcl: %v", err)
	}
	var ids []string
	for _, rule := range rules {
		ids = append(ids, rule.Id)
	}
	want := []string{"user:one@example.com", "user:two@example.com"}
	if len(ids) != len(want) {
		t.Fatalf("ListAcl rule ids = %v, want %v", ids, want)
	}
	for i, id := range want {
		if ids[i] != id {
			t.Errorf("rules[%d].Id = %s, want %s", i, ids[i], id)
		}
	}
}

// pageAllAllPages walks a two-page fetch and proves pageAll concatenates
// every page, not just the first.
func TestPageAllFollowsAllPages(t *testing.T) {
	fetched := 0
	items, err := pageAll(func(page string) ([]string, string, error) {
		fetched++
		switch fetched {
		case 1:
			return []string{"a", "b"}, "tok-2", nil
		case 2:
			return []string{"c"}, "", nil
		default:
			t.Errorf("unexpected page %d", fetched)
			return nil, "", nil
		}
	})
	if err != nil {
		t.Fatalf("pageAll: %v", err)
	}
	if fetched != 2 {
		t.Errorf("pages fetched = %d, want 2", fetched)
	}
	got := ""
	for _, it := range items {
		got += it
	}
	if got != "abc" {
		t.Errorf("items = %v, want [a b c]", items)
	}
}

// pageAllTerminateGuards the safety cap: a fetch that always advertises a
// next token must error out instead of looping forever.
func TestPageAllTerminatesOnRunawayToken(t *testing.T) {
	_, err := pageAll(func(page string) ([]string, string, error) {
		return []string{"x"}, "never-empty", nil
	})
	if err == nil {
		t.Fatal("pageAll: want error on runaway NextPageToken, got nil")
	}
}

// pageAllBudgetedStopsAtBudget proves the item budget halts pagination: a
// 3-item-per-page fetch with a budget of 5 collects exactly 5 items and never
// fetches the page beyond the budget (page two's overshoot is truncated).
func TestPageAllBudgetedStopsAtBudget(t *testing.T) {
	var sawRemaining []int64
	fetched := 0
	items, err := pageAllBudgeted(5, func(page string, remaining int64) ([]string, string, error) {
		fetched++
		sawRemaining = append(sawRemaining, remaining)
		switch fetched {
		case 1:
			return []string{"a", "b", "c"}, "tok-2", nil
		case 2:
			return []string{"d", "e", "f"}, "tok-3", nil // overshoots remaining 2
		default:
			t.Errorf("unexpected fetch #%d past the budget", fetched)
			return nil, "", nil
		}
	})
	if err != nil {
		t.Fatalf("pageAllBudgeted: %v", err)
	}
	if fetched != 2 {
		t.Errorf("pages fetched = %d, want 2 (no fetch beyond the budget page)", fetched)
	}
	if len(items) != 5 {
		t.Errorf("items = %v, want exactly 5", items)
	}
	wantRemaining := []int64{5, 2}
	for i, want := range wantRemaining {
		if sawRemaining[i] != want {
			t.Errorf("remaining on fetch %d = %d, want %d", i, sawRemaining[i], want)
		}
	}
}

// pageAllBudgetedUnlimited pages to the end like pageAll: budget <= 0 must
// keep the full-listing behavior (never stops early, never truncates).
func TestPageAllBudgetedUnlimitedRunsToTheEnd(t *testing.T) {
	fetched := 0
	items, err := pageAllBudgeted(0, func(page string, remaining int64) ([]string, string, error) {
		fetched++
		if remaining != 0 {
			t.Errorf("remaining = %d, want 0 for an unlimited listing", remaining)
		}
		switch fetched {
		case 1:
			return []string{"a", "b"}, "tok-2", nil
		case 2:
			return []string{"c"}, "", nil
		default:
			t.Errorf("unexpected page %d", fetched)
			return nil, "", nil
		}
	})
	if err != nil {
		t.Fatalf("pageAllBudgeted: %v", err)
	}
	if fetched != 2 {
		t.Errorf("pages fetched = %d, want 2", fetched)
	}
	if len(items) != 3 {
		t.Errorf("items = %v, want [a b c]", items)
	}
}

// TestListEventsBudgetStopsPagination drives the real ListEvents over a
// three-page fake REST server with a budget of 5: exactly 5 events come
// back, and the third page is never fetched (the server 400s on it). The
// per-request maxResults must also track the remaining budget.
func TestListEventsBudgetStopsPagination(t *testing.T) {
	var fetches []string // pageToken of each fetch, in order
	svc := newPagedTestServer(t, map[string]http.HandlerFunc{
		"/calendars/cal-1/events": func(w http.ResponseWriter, r *http.Request) {
			fetches = append(fetches, r.URL.Query().Get("pageToken"))
			switch r.URL.Query().Get("pageToken") {
			case "":
				writeJSON(w, calendar.Events{
					Items:         []*calendar.Event{{Id: "ev-1"}, {Id: "ev-2"}, {Id: "ev-3"}},
					NextPageToken: "tok-2",
				})
			case "tok-2":
				writeJSON(w, calendar.Events{
					Items:         []*calendar.Event{{Id: "ev-4"}, {Id: "ev-5"}, {Id: "ev-6"}},
					NextPageToken: "tok-3",
				})
			default:
				http.Error(w, "fetched beyond the budget", http.StatusBadRequest)
			}
		},
	})

	events, err := svc.ListEvents(t.Context(), ListEventsParams{CalendarID: "cal-1", MaxResults: 5})
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if len(fetches) != 2 {
		t.Fatalf("page fetches = %v, want 2 (no fetch beyond the budget)", fetches)
	}
	var ids []string
	for _, ev := range events {
		ids = append(ids, ev.Id)
	}
	want := []string{"ev-1", "ev-2", "ev-3", "ev-4", "ev-5"}
	if len(ids) != len(want) {
		t.Fatalf("event ids = %v, want %v", ids, want)
	}
	for i, id := range want {
		if ids[i] != id {
			t.Errorf("events[%d].Id = %s, want %s", i, ids[i], id)
		}
	}
}
