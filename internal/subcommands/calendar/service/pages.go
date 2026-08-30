package service

import "fmt"

// maxListPages caps how many pages one listing may follow before giving up.
// A well-behaved server ends with NextPageToken == "", so the cap only ever
// fires on a misbehaving (or adversarial) endpoint looping tokens forever.
// 1000 pages is far beyond any real listing.
const maxListPages = 1000

// fetchPage fetches one page of a paginated Calendar list call: page is the
// NextPageToken of the previous response ("" for the first page). It returns
// the page's items and the token for the next page ("" when the listing is
// exhausted). Errors are fully wrapped by the caller-specific fetch, so
// pageAll returns them as-is.
type fetchPage[T any] func(page string) (items []T, nextPageToken string, err error)

// pageAll pages a Calendar list endpoint until NextPageToken is empty and
// returns every item across all pages. Unlike Gmail's collectPages it takes
// no maxResults budget — the Calendar endpoints paged here (CalendarList,
// Acl, Events) either have no meaningful item budget (full listing) or are
// already bounded by their own query params.
func pageAll[T any](fetch fetchPage[T]) ([]T, error) {
	var items []T
	for page, pages := "", 0; ; pages++ {
		paged, next, err := fetch(page)
		if err != nil {
			return nil, err
		}
		items = append(items, paged...)
		if next == "" {
			return items, nil
		}
		if pages >= maxListPages {
			return nil, fmt.Errorf("listing did not terminate after %d pages", maxListPages)
		}
		page = next
	}
}
