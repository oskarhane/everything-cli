package service

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"golang.org/x/oauth2"
	drive "google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

// newPagedTestServer builds a hermetic fake of the Drive REST endpoints
// listed in handlers (path → fetch handler) and returns a realDriveService
// pointed at it. Each handler sees pageToken; responses are plain JSON
// fragments served verbatim, so the tests below spell out the raw two-page
// shape (items + nextPageToken on the first page, "" on the second).
func newPagedTestServer(t *testing.T, handlers map[string]http.HandlerFunc) *realDriveService {
	t.Helper()
	mux := http.NewServeMux()
	for path, h := range handlers {
		mux.HandleFunc(path, h)
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	svc, err := drive.NewService(t.Context(),
		option.WithEndpoint(srv.URL),
		option.WithTokenSource(oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test-token"})),
	)
	if err != nil {
		t.Fatalf("creating drive service: %v", err)
	}
	return &realDriveService{drive: svc}
}

// writeJSON serves v as a 200 JSON response.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// TestListFilesFollowsAllPages drives the real ListFiles over a two-page
// fake files.list endpoint: all three files come back, page two's token is
// chained, and the query is passed through verbatim.
func TestListFilesFollowsAllPages(t *testing.T) {
	var sawQuery []string
	svc := newPagedTestServer(t, map[string]http.HandlerFunc{
		"/files": func(w http.ResponseWriter, r *http.Request) {
			sawQuery = append(sawQuery, r.URL.Query().Get("q"))
			switch r.URL.Query().Get("pageToken") {
			case "":
				writeJSON(w, drive.FileList{
					Files:         []*drive.File{{Id: "f-1"}, {Id: "f-2"}},
					NextPageToken: "tok-2",
				})
			case "tok-2":
				writeJSON(w, drive.FileList{
					Files:         []*drive.File{{Id: "f-3"}},
					NextPageToken: "",
				})
			default:
				http.Error(w, "unexpected pageToken", http.StatusBadRequest)
			}
		},
	})

	files, err := svc.ListFiles(t.Context(), "mimeType != 'application/vnd.google-apps.folder'", 0)
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}
	if sawQuery[0] != "mimeType != 'application/vnd.google-apps.folder'" {
		t.Errorf("query sent = %q, want the caller's query verbatim", sawQuery[0])
	}
	var ids []string
	for _, f := range files {
		ids = append(ids, f.Id)
	}
	// Without paging this returns only f-1 and f-2: page one is the last one
	// seen. Asserting all three proves the truncation is gone.
	want := []string{"f-1", "f-2", "f-3"}
	if len(ids) != len(want) {
		t.Fatalf("file ids = %v, want %v", ids, want)
	}
	for i, id := range want {
		if ids[i] != id {
			t.Errorf("files[%d].Id = %s, want %s", i, ids[i], id)
		}
	}
}

// TestListFilesBudgetStopsPagination drives ListFiles over a three-page fake
// with a budget of 5: exactly 5 files come back, the third page is never
// fetched (the server 400s on it), and the per-request page size tracks the
// remaining budget.
func TestListFilesBudgetStopsPagination(t *testing.T) {
	var (
		fetches []string // pageToken of each fetch, in order
		sawSize []string
	)
	svc := newPagedTestServer(t, map[string]http.HandlerFunc{
		"/files": func(w http.ResponseWriter, r *http.Request) {
			fetches = append(fetches, r.URL.Query().Get("pageToken"))
			sawSize = append(sawSize, r.URL.Query().Get("pageSize"))
			switch r.URL.Query().Get("pageToken") {
			case "":
				writeJSON(w, drive.FileList{
					Files:         []*drive.File{{Id: "f-1"}, {Id: "f-2"}, {Id: "f-3"}},
					NextPageToken: "tok-2",
				})
			case "tok-2":
				writeJSON(w, drive.FileList{
					Files:         []*drive.File{{Id: "f-4"}, {Id: "f-5"}, {Id: "f-6"}},
					NextPageToken: "tok-3",
				})
			default:
				http.Error(w, "fetched beyond the budget", http.StatusBadRequest)
			}
		},
	})

	files, err := svc.ListFiles(t.Context(), "", 5)
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}
	if len(fetches) != 2 {
		t.Fatalf("page fetches = %v, want 2 (no fetch beyond the budget)", fetches)
	}
	if sawSize[0] != "5" {
		t.Errorf("page one pageSize = %q, want 5 (full remaining budget)", sawSize[0])
	}
	if sawSize[1] != "2" {
		t.Errorf("page two pageSize = %q, want 2 (remaining after page one)", sawSize[1])
	}
	var ids []string
	for _, f := range files {
		ids = append(ids, f.Id)
	}
	want := []string{"f-1", "f-2", "f-3", "f-4", "f-5"}
	if len(ids) != len(want) {
		t.Fatalf("file ids = %v, want %v", ids, want)
	}
	for i, id := range want {
		if ids[i] != id {
			t.Errorf("files[%d].Id = %s, want %s", i, ids[i], id)
		}
	}
}

// TestListPermissionsFollowsAllPages drives ListPermissions over a fake
// permissions.list: both pages' rules must come back.
func TestListPermissionsFollowsAllPages(t *testing.T) {
	svc := newPagedTestServer(t, map[string]http.HandlerFunc{
		"/files/f-1/permissions": func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Query().Get("pageToken") {
			case "":
				writeJSON(w, drive.PermissionList{
					Permissions:   []*drive.Permission{{Id: "perm-1", Type: "user"}},
					NextPageToken: "tok-2",
				})
			case "tok-2":
				writeJSON(w, drive.PermissionList{
					Permissions:   []*drive.Permission{{Id: "perm-2", Type: "anyone"}},
					NextPageToken: "",
				})
			default:
				http.Error(w, "unexpected pageToken", http.StatusBadRequest)
			}
		},
	})

	perms, err := svc.ListPermissions(t.Context(), "f-1")
	if err != nil {
		t.Fatalf("ListPermissions: %v", err)
	}
	var ids []string
	for _, p := range perms {
		ids = append(ids, p.Id)
	}
	want := []string{"perm-1", "perm-2"}
	if len(ids) != len(want) {
		t.Fatalf("permission ids = %v, want %v", ids, want)
	}
	for i, id := range want {
		if ids[i] != id {
			t.Errorf("perms[%d].Id = %s, want %s", i, ids[i], id)
		}
	}
}

// pageAllFollowsAllPages walks a two-page fetch and proves pageAll
// concatenates every page, not just the first.
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

// pageAllBudgetedClampsPerPageRequest: the remaining budget passed to fetch
// shrinks by the pages actually returned and is clamped to maxFilePageSize:
// budget 1200 with pages that return exactly what was asked must see
// requests of 1000, 200.
func TestPageAllBudgetedClampsPerPageRequest(t *testing.T) {
	var sawRemaining []int64
	items, err := pageAllBudgeted(1200, func(page string, remaining int64) ([]string, string, error) {
		sawRemaining = append(sawRemaining, remaining)
		return make([]string, remaining), "next", nil
	})
	if err != nil {
		t.Fatalf("pageAllBudgeted: %v", err)
	}
	if len(items) != 1200 {
		t.Fatalf("len(items) = %d, want 1200", len(items))
	}
	want := []int64{1000, 200}
	if !reflect.DeepEqual(sawRemaining, want) {
		t.Fatalf("remaining values seen = %v, want %v", sawRemaining, want)
	}
}

// pageAllBudgetedErrorPropagatesUnwrapped guards the helper contract: fetch
// wraps its own errors, so pageAllBudgeted must return the error as-is.
func TestPageAllBudgetedErrorPropagatesUnwrapped(t *testing.T) {
	sentinel := errors.New("listing files: api exploded")
	_, err := pageAllBudgeted[string](5, func(page string, remaining int64) ([]string, string, error) {
		return nil, "", sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want the fetch error unwrapped", err)
	}
}
