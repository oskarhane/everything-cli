package slack

import (
	"context"
	"net/url"
	"strconv"
	"strings"
)

// usersListPageSize is the per-request page size asked of users.list. Slack
// caps the page at 200 members; the cursor drives the rest.
const usersListPageSize = 200

// usersInfoResponse is the pinned subset of GET /users.info: the one member
// object under "user".
type usersInfoResponse struct {
	User wireUser `json:"user"`
}

// usersListResponse is the pinned subset of GET /users.list: the member page
// plus the response_metadata cursor that drives pagination.
type usersListResponse struct {
	cursorEnvelope
	Members []wireUser `json:"members"`
}

// UserGet returns the workspace member with the given Slack user ID
// (users.info). display_name comes from the nested profile object; real_name
// comes from the top-level user object, which Slack populates even when the
// profile is absent.
func (s *httpService) UserGet(ctx context.Context, userID string) (User, error) {
	q := url.Values{}
	q.Set("user", userID)
	var out usersInfoResponse
	if err := s.apiCall(ctx, "/users.info", q, &out); err != nil {
		return User{}, err
	}
	return out.User.view(), nil
}

// UserList returns workspace members (users.list), following
// response_metadata.next_cursor across pages. query filters client-side,
// case-insensitively, over name, real_name, and display_name; max caps the
// matches returned and is 0 for no cap. Paging stops as soon as max matches
// are collected, so a query that never matches pages every member. The filter
// lives inside the fetch closure, so it sees every raw page while the
// collector's budget counts only the matches; requests therefore always ask
// for the full usersListPageSize.
func (s *httpService) UserList(ctx context.Context, query string, max int) ([]User, error) {
	return collectCursorPages(ctx, max, "user listing",
		func(ctx context.Context, cursor string, _ int) ([]User, string, error) {
			q := url.Values{}
			q.Set("limit", strconv.Itoa(usersListPageSize))
			if cursor != "" {
				q.Set("cursor", cursor)
			}
			var page usersListResponse
			if err := s.apiCall(ctx, "/users.list", q, &page); err != nil {
				return nil, "", err
			}
			users := make([]User, 0, len(page.Members))
			for _, member := range page.Members {
				u := member.view()
				if !userMatch(u, query) {
					continue
				}
				users = append(users, u)
			}
			return users, page.nextCursor(), nil
		})
}

// userMatch reports whether u matches query: a case-insensitive substring of
// its name, real_name, or display_name. An empty query matches every member.
func userMatch(u User, query string) bool {
	if query == "" {
		return true
	}
	q := strings.ToLower(query)
	return strings.Contains(strings.ToLower(u.Name), q) ||
		strings.Contains(strings.ToLower(u.RealName), q) ||
		strings.Contains(strings.ToLower(u.DisplayName), q)
}
