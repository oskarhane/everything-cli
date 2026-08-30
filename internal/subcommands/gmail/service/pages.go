package service

// maxPageSize is the Gmail API's per-request cap on maxResults.
const maxPageSize = 500

// fetchPage performs one page of a paginated Gmail list call: page is the
// NextPageToken of the previous response ("" for the first page) and
// remaining is the number of items still to fetch, already clamped by
// collectPages to maxPageSize. It returns the page's items and the token for
// the next page ("" when the listing is exhausted). Errors are fully wrapped
// by the caller-specific fetch, so collectPages returns them as-is.
type fetchPage[T any] func(page string, remaining int64) (items []T, nextPageToken string, err error)

// collectPages pages a Gmail list endpoint until maxResults items are
// collected or the pages run out, then truncates any overshoot (a page may
// return more items than the remaining budget asked for). The per-request
// MaxResults value is clamped to maxPageSize.
//
// Callers supply maxResults (already defaulted if <= 0) and a fetch that
// wraps one endpoint's list call — query params, label filters and error
// messages differ per endpoint, the loop does not.
func collectPages[T any](maxResults int64, fetch fetchPage[T]) ([]T, error) {
	var items []T
	for page := ""; ; {
		paged, next, err := fetch(page, min(maxResults-int64(len(items)), maxPageSize))
		if err != nil {
			return nil, err
		}
		items = append(items, paged...)
		if next == "" || int64(len(items)) >= maxResults {
			break
		}
		page = next
	}
	if int64(len(items)) > maxResults {
		items = items[:maxResults]
	}
	return items, nil
}
