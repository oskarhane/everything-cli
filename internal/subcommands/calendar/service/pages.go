package service

import "fmt"

// maxListPages caps how many pages one listing may follow before giving up.
// A well-behaved server ends with NextPageToken == "", so the cap only ever
// fires on a misbehaving (or adversarial) endpoint looping tokens forever.
// 1000 pages is far beyond any real listing.
const maxListPages = 1000

// maxEventPageSize is the Calendar API's per-request cap on maxResults for
// the event endpoints (events.list, events.instances).
const maxEventPageSize = 2500

// fetchPage fetches one page of a paginated Calendar list call: page is the
// NextPageToken of the previous response ("" for the first page). It returns
// the page's items and the token for the next page ("" when the listing is
// exhausted). Errors are fully wrapped by the caller-specific fetch, so
// pageAll returns them as-is.
type fetchPage[T any] func(page string) (items []T, nextPageToken string, err error)

// budgetedFetchPage is fetchPage with a remaining-item budget. remaining is
// 0 for an unlimited listing (fetch must then leave the request's page size
// at the API default); otherwise it is the number of items still to fetch,
// already clamped by pageAllBudgeted to maxEventPageSize.
type budgetedFetchPage[T any] func(page string, remaining int64) (items []T, nextPageToken string, err error)

// pageAllBudgeted pages a Calendar list endpoint until maxResults items are
// collected or the pages run out, then truncates any overshoot (a page may
// return more items than the remaining budget asked for). The per-request
// MaxResults value is clamped to maxEventPageSize. maxResults <= 0 means no
// budget: fetch sees remaining == 0 and the listing runs until the token is
// empty (still under the maxListPages runaway cap).
func pageAllBudgeted[T any](maxResults int64, fetch budgetedFetchPage[T]) ([]T, error) {
	var items []T
	for page, pages := "", 0; ; pages++ {
		remaining := int64(0)
		if maxResults > 0 {
			remaining = min(maxResults-int64(len(items)), maxEventPageSize)
		}
		paged, next, err := fetch(page, remaining)
		if err != nil {
			return nil, err
		}
		items = append(items, paged...)
		if next == "" || (maxResults > 0 && int64(len(items)) >= maxResults) {
			break
		}
		if pages >= maxListPages {
			return nil, fmt.Errorf("listing did not terminate after %d pages", maxListPages)
		}
		page = next
	}
	if maxResults > 0 && int64(len(items)) > maxResults {
		items = items[:maxResults]
	}
	return items, nil
}

// pageAll pages a Calendar list endpoint until NextPageToken is empty and
// returns every item across all pages. It takes no maxResults budget — the
// Calendar endpoints paged without one (CalendarList, Acl) have no
// meaningful item budget (full listing). Event listing uses pageAllBudgeted
// instead, so its callers cap the total with --max.
func pageAll[T any](fetch fetchPage[T]) ([]T, error) {
	return pageAllBudgeted(0, func(page string, _ int64) ([]T, string, error) {
		return fetch(page)
	})
}
