package service

import (
	"errors"
	"reflect"
	"testing"
)

func TestCollectPages_ClampsPerPageRequest(t *testing.T) {
	// The remaining budget passed to fetch shrinks by the pages actually
	// returned and is clamped to maxPageSize: budget 1200 with pages that
	// return exactly what was asked must see requests of 500, 500, 200.
	var sawRemaining []int64
	items, err := collectPages(1200, func(page string, remaining int64) ([]string, string, error) {
		sawRemaining = append(sawRemaining, remaining)
		return make([]string, remaining), "next", nil
	})
	if err != nil {
		t.Fatalf("collectPages: %v", err)
	}
	if len(items) != 1200 {
		t.Fatalf("len(items) = %d, want 1200", len(items))
	}
	want := []int64{500, 500, 200}
	if !reflect.DeepEqual(sawRemaining, want) {
		t.Fatalf("remaining values seen = %v, want %v", sawRemaining, want)
	}
}

func TestCollectPages_ChainsPagesViaNextToken(t *testing.T) {
	// Pages chain by nextToken until "", all items accumulated in page order.
	pages := [][]string{
		{"p1-a", "p1-b", "p1-c"},
		{"p2-a", "p2-b"},
		{"p3-a"},
	}
	tokens := []string{"t2", "t3", ""}
	var gotTokens []string
	items, err := collectPages(100, func(page string, remaining int64) ([]string, string, error) {
		gotTokens = append(gotTokens, page)
		i := len(gotTokens) - 1
		return pages[i], tokens[i], nil
	})
	if err != nil {
		t.Fatalf("collectPages: %v", err)
	}
	want := []string{"p1-a", "p1-b", "p1-c", "p2-a", "p2-b", "p3-a"}
	if !reflect.DeepEqual(items, want) {
		t.Fatalf("items = %v, want %v", items, want)
	}
	if !reflect.DeepEqual(gotTokens, []string{"", "t2", "t3"}) {
		t.Fatalf("page tokens = %v, want [\"\" t2 t3]", gotTokens)
	}
}

func TestCollectPages_TruncatesOvershootToMaxResults(t *testing.T) {
	// A page may return more items than the remaining budget asked for: the
	// result is truncated to exactly maxResults.
	items, err := collectPages(10, func(page string, remaining int64) ([]string, string, error) {
		return []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m"}, "next", nil
	})
	if err != nil {
		t.Fatalf("collectPages: %v", err)
	}
	if len(items) != 10 {
		t.Fatalf("len(items) = %d, want 10 (truncated to maxResults)", len(items))
	}
	if items[9] != "j" {
		t.Fatalf("items[9] = %q, want \"j\"", items[9])
	}
}

func TestCollectPages_ErrorPropagatesUnwrapped(t *testing.T) {
	// collectPages returns the fetch error as-is: callers wrap in their own
	// fetch, so the helper must not add another layer.
	sentinel := errors.New("listing messages: api exploded")
	_, err := collectPages[string](5, func(page string, remaining int64) ([]string, string, error) {
		return nil, "", sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want the fetch error unwrapped", err)
	}
}

func TestCollectPages_EmptyListing(t *testing.T) {
	// An endpoint with no results: one fetch returning nothing and "" ends
	// the loop with zero items and no error.
	items, err := collectPages[string](100, func(page string, remaining int64) ([]string, string, error) {
		return nil, "", nil
	})
	if err != nil {
		t.Fatalf("collectPages: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("items = %v, want empty", items)
	}
}
