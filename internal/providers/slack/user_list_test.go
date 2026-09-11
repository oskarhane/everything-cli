package slack

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// userListPage2Cursor is the response_metadata.next_cursor recorded in
// testdata/user_list_page1.json.
const userListPage2Cursor = "dXNlcnM6cGFnZTI="

// newUserListServer serves the two recorded users.list pages keyed by the
// cursor param and records every request.
func newUserListServer(t *testing.T, log *userRequestLog) *httptest.Server {
	t.Helper()
	page1 := userFixture(t, "user_list_page1.json")
	page2 := userFixture(t, "user_list_page2.json")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.add(r)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/users.list" && r.URL.Query().Get("cursor") == "":
			_, _ = w.Write(page1)
		case r.URL.Path == "/users.list" && r.URL.Query().Get("cursor") == userListPage2Cursor:
			_, _ = w.Write(page2)
		default:
			http.Error(w, `{"ok":false,"error":"invalid_cursor"}`, http.StatusBadRequest)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// runUserList runs `slack user list --format json` against srv and returns
// the decoded rows in order. Output's one-row collapse means a single match
// arrives as an object rather than a one-element array; both decode.
func runUserList(t *testing.T, srv *httptest.Server, args ...string) (string, []map[string]any) {
	t.Helper()
	_, root, out := newSlackEnv(t)
	stubDial(t, newHTTPService(srv.Client(), srv.URL))
	stdout, err := execute(t, root, out, append([]string{"slack", "user", "list", "--format", "json"}, args...)...)
	require.NoError(t, err)

	var decoded any
	require.NoError(t, json.Unmarshal([]byte(stdout), &decoded))
	switch v := decoded.(type) {
	case []any:
		rows := make([]map[string]any, 0, len(v))
		for _, item := range v {
			row, ok := item.(map[string]any)
			require.True(t, ok, "every user list element must be an object")
			rows = append(rows, row)
		}
		return stdout, rows
	case map[string]any:
		return stdout, []map[string]any{v}
	default:
		t.Fatalf("unexpected user list JSON shape %T", decoded)
		return "", nil
	}
}

// userIDs projects rows onto their id column, keeping order.
func userIDs(rows []map[string]any) []string {
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		id, _ := row["id"].(string)
		ids = append(ids, id)
	}
	return ids
}

// TestUserListPaginatesAcrossPages: the default listing follows
// response_metadata.next_cursor and concatenates both pages in order, asking
// for the capped page size with no cursor first and the page cursor second.
func TestUserListPaginatesAcrossPages(t *testing.T) {
	var log userRequestLog
	srv := newUserListServer(t, &log)

	_, rows := runUserList(t, srv)
	assert.Equal(t, []string{
		"U02H6ECK2", "U0ALICE01", "U0OZZY01",
		"U0CAROL01", "U0OZZY02", "U0QUINN01", "U0REAL01",
	}, userIDs(rows))

	queries := log.all()
	require.Len(t, queries, 2)
	assert.Equal(t, []string{"200"}, queries[0]["limit"])
	assert.Empty(t, queries[0]["cursor"])
	assert.Equal(t, []string{"200"}, queries[1]["limit"])
	assert.Equal(t, []string{userListPage2Cursor}, queries[1]["cursor"])
}

// TestUserListQueryFiltersAcrossPages: --query filters client-side,
// case-insensitively. U0OZZY01 matches only through display_name
// ("OZZY-fan", name "bob", real_name "Bob Berg"); U0OZZY02 lives on page
// two, so the display_name-only match must not stop the cursor loop.
func TestUserListQueryFiltersAcrossPages(t *testing.T) {
	var log userRequestLog
	srv := newUserListServer(t, &log)

	_, rows := runUserList(t, srv, "--query", "ozzy")
	assert.Equal(t, []string{"U0OZZY01", "U0OZZY02"}, userIDs(rows))
	require.Len(t, rows, 2)
	assert.Equal(t, "bob", rows[0]["name"])
	assert.Equal(t, "Bob Berg", rows[0]["real_name"])
	assert.Equal(t, "OZZY-fan", rows[0]["display_name"])
	require.Len(t, log.all(), 2, "a page-one match must not stop paging when the listing continues")
}

// TestUserListQueryMatchesEachFieldCaseInsensitively pins the filter over
// all three fields: a query matching only name, only real_name, or only
// display_name each returns its member regardless of case.
func TestUserListQueryMatchesEachFieldCaseInsensitively(t *testing.T) {
	cases := []struct {
		name  string
		query string
		want  []string
	}{
		{"name only", "QUINN", []string{"U0QUINN01"}},
		{"real_name only", "uniquer", []string{"U0REAL01"}},
		{"display_name only", "ozzy-fan", []string{"U0OZZY01"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var log userRequestLog
			srv := newUserListServer(t, &log)

			_, rows := runUserList(t, srv, "--query", tc.query)
			assert.Equal(t, tc.want, userIDs(rows))
		})
	}
}

// TestUserListMaxCapsMatchingUsers: --max counts matching members, not raw
// page members, and stops the cursor loop as soon as enough matches are
// collected; --max 0 follows every page.
func TestUserListMaxCapsMatchingUsers(t *testing.T) {
	t.Run("max 1 stops before page two", func(t *testing.T) {
		var log userRequestLog
		srv := newUserListServer(t, &log)

		_, rows := runUserList(t, srv, "--query", "ozzy", "--max", "1")
		assert.Equal(t, []string{"U0OZZY01"}, userIDs(rows))
		assert.Len(t, log.all(), 1, "page two must not be fetched once --max matches are collected")
	})

	t.Run("max 0 follows every page", func(t *testing.T) {
		var log userRequestLog
		srv := newUserListServer(t, &log)

		_, rows := runUserList(t, srv, "--query", "ozzy", "--max", "0")
		assert.Equal(t, []string{"U0OZZY01", "U0OZZY02"}, userIDs(rows))
		assert.Len(t, log.all(), 2)
	})
}

// TestUserListNoMatchIsEmpty: a query matching nothing still pages the
// listing out and renders an empty array.
func TestUserListNoMatchIsEmpty(t *testing.T) {
	var log userRequestLog
	srv := newUserListServer(t, &log)

	stdout, rows := runUserList(t, srv, "--query", "does-not-exist")
	assert.Empty(t, rows)
	assert.Equal(t, "[]", strings.TrimSpace(stdout))
	assert.Len(t, log.all(), 2, "a non-matching page still follows the cursor")
}

// TestUserListTable pins the UPPER-CASE table headers of the shared user
// output shape.
func TestUserListTable(t *testing.T) {
	var log userRequestLog
	srv := newUserListServer(t, &log)
	_, root, out := newSlackEnv(t)
	stubDial(t, newHTTPService(srv.Client(), srv.URL))

	stdout, err := execute(t, root, out, "slack", "user", "list", "--query", "oskar", "--format", "table")
	require.NoError(t, err)
	for _, header := range []string{"ID", "NAME", "REAL_NAME", "DISPLAY_NAME"} {
		assert.Contains(t, stdout, header)
	}
	assert.Contains(t, stdout, "U02H6ECK2")
	assert.Contains(t, stdout, "oskarh")
}

// TestUserListAPIOKFalseCarriesCode: an HTTP-200 ok:false answer surfaces
// the raw Slack error code in the command error.
func TestUserListAPIOKFalseCarriesCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":false,"error":"invalid_cursor"}`))
	}))
	t.Cleanup(srv.Close)
	_, root, out := newSlackEnv(t)
	stubDial(t, newHTTPService(srv.Client(), srv.URL))

	_, err := execute(t, root, out, "slack", "user", "list", "--format", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid_cursor")
}
