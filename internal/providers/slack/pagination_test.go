package slack

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// scriptedPage is one page a test fetch closure serves, keyed by the cursor
// that requests it.
type scriptedPage struct {
	items []int
	next  string
}

// fetchCall records one collector -> fetch call: the cursor requested and the
// remaining-budget hint the collector passed down.
type fetchCall struct {
	cursor string
	limit  int
}

// scriptedFetch returns a fetch closure over pages that records every call.
func scriptedFetch(t *testing.T, pages map[string]scriptedPage, calls *[]fetchCall) func(context.Context, string, int) ([]int, string, error) {
	t.Helper()
	return func(_ context.Context, cursor string, limit int) ([]int, string, error) {
		*calls = append(*calls, fetchCall{cursor: cursor, limit: limit})
		page, ok := pages[cursor]
		require.True(t, ok, "unexpected cursor %q", cursor)
		return page.items, page.next, nil
	}
}

// TestCollectCursorPagesFollowsCursors: the collector threads each next cursor
// into the following fetch and stops on the empty one; an uncapped listing
// passes the 0 hint (ask for a full page).
func TestCollectCursorPagesFollowsCursors(t *testing.T) {
	var calls []fetchCall
	fetch := scriptedFetch(t, map[string]scriptedPage{
		"":   {items: []int{1, 2}, next: "p2"},
		"p2": {items: []int{3}, next: ""},
	}, &calls)

	items, err := collectCursorPages(context.Background(), 0, "test listing", fetch)
	require.NoError(t, err)
	assert.Equal(t, []int{1, 2, 3}, items)
	assert.Equal(t, []fetchCall{{cursor: "", limit: 0}, {cursor: "p2", limit: 0}}, calls)
}

// TestCollectCursorPagesCapsAndTruncates: max stops the loop as soon as the
// budget is filled, hands each fetch the remaining budget, and trims the last
// page's overshoot.
func TestCollectCursorPagesCapsAndTruncates(t *testing.T) {
	var calls []fetchCall
	fetch := scriptedFetch(t, map[string]scriptedPage{
		"":   {items: []int{1, 2}, next: "p2"},
		"p2": {items: []int{3, 4}, next: "p3"},
	}, &calls)

	items, err := collectCursorPages(context.Background(), 3, "test listing", fetch)
	require.NoError(t, err)
	assert.Equal(t, []int{1, 2, 3}, items, "the fourth item is truncated")
	assert.Equal(t, []fetchCall{{cursor: "", limit: 3}, {cursor: "p2", limit: 1}}, calls, "p3 is never fetched")
}

// TestCollectCursorPagesRunawayCursorIsAnError: a cursor that never empties
// stops at maxListPages with the labelled error and no partial items.
func TestCollectCursorPagesRunawayCursorIsAnError(t *testing.T) {
	var calls []fetchCall
	fetch := func(_ context.Context, cursor string, limit int) ([]int, string, error) {
		calls = append(calls, fetchCall{cursor: cursor, limit: limit})
		return []int{1}, "always", nil
	}

	items, err := collectCursorPages(context.Background(), 0, "test listing", fetch)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "test listing did not terminate after 100 pages")
	assert.Nil(t, items, "a runaway cursor returns no partial output")
	require.Len(t, calls, maxListPages)
	assert.Equal(t, "always", calls[1].cursor, "the first page asks with the empty cursor")
}

// TestCollectCursorPagesPropagatesFetchError: a fetch failure aborts the
// listing with that error, not a wrapped or swallowed one.
func TestCollectCursorPagesPropagatesFetchError(t *testing.T) {
	want := errors.New("boom")
	fetch := func(context.Context, string, int) ([]int, string, error) {
		return nil, "", want
	}

	items, err := collectCursorPages(context.Background(), 0, "test listing", fetch)
	require.ErrorIs(t, err, want)
	assert.Nil(t, items)
}

// TestPageLimitClampsToBudget pins the per-request page size helper the
// listings feed the collector's budget hint into.
func TestPageLimitClampsToBudget(t *testing.T) {
	assert.Equal(t, 200, pageLimit(200, 0), "no cap asks for a full page")
	assert.Equal(t, 4, pageLimit(200, 4), "a smaller budget wins")
	assert.Equal(t, 200, pageLimit(200, 500), "a larger budget leaves the full page")
}
