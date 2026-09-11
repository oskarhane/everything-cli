package slack

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHTTPServiceClientTimeout pins the request timeout: a client without
// one gets apiTimeout so a hung endpoint cannot stall a command; an explicit
// timeout is preserved.
func TestHTTPServiceClientTimeout(t *testing.T) {
	svc := newHTTPService(&http.Client{}, "http://example.invalid")
	assert.Equal(t, apiTimeout, svc.client.Timeout)

	custom := 5 * time.Second
	svc = newHTTPService(&http.Client{Timeout: custom}, "http://example.invalid")
	assert.Equal(t, custom, svc.client.Timeout)
}

// newErrorService serves one fixed JSON body, so tests exercise the real
// apiCall mapping hermetically.
func newErrorService(t *testing.T, body string) *httpService {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return newHTTPService(srv.Client(), srv.URL)
}

// TestAPICallOKFalseCarriesCode pins the three error codes an agent actually
// hits: HTTP 200 with ok:false must surface the raw code verbatim.
func TestAPICallOKFalseCarriesCode(t *testing.T) {
	cases := []struct {
		name string
		code string
	}{
		{"missing scope", "missing_scope"},
		{"not allowed token type", "not_allowed_token_type"},
		{"channel not found", "channel_not_found"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := newErrorService(t, fmt.Sprintf(`{"ok":false,"error":%q}`, tc.code))

			err := svc.apiCall(context.Background(), "/conversations.list", nil, &struct{}{})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.code)

			var apiErr *apiError
			require.ErrorAs(t, err, &apiErr)
			assert.Equal(t, tc.code, apiErr.Code)
		})
	}
}

// TestAPICallOKFalseCarriesScopeDetail: missing_scope answers carry the
// needed and provided scopes; both must reach the error message.
func TestAPICallOKFalseCarriesScopeDetail(t *testing.T) {
	svc := newErrorService(t, `{"ok":false,"error":"missing_scope","needed":"channels:history","provided":"channels:read"}`)

	err := svc.apiCall(context.Background(), "/conversations.history", nil, &struct{}{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing_scope")
	assert.Contains(t, err.Error(), "channels:history")
	assert.Contains(t, err.Error(), "channels:read")

	var apiErr *apiError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, "channels:history", apiErr.Needed)
	assert.Equal(t, "channels:read", apiErr.Provided)
}

// TestAPICallRateLimitSurfacesRetryAfter: a 429 with Retry-After must name
// the seconds and make exactly one request — there is no retry machinery.
func TestAPICallRateLimitSurfacesRetryAfter(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"ok":false,"error":"ratelimited"}`))
	}))
	t.Cleanup(srv.Close)
	svc := newHTTPService(srv.Client(), srv.URL)

	err := svc.apiCall(context.Background(), "/conversations.list", nil, &struct{}{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "retry after 30s")
	assert.EqualValues(t, 1, calls.Load())
}

// TestAPICallDecodesNonStrictly: Slack payloads are subtype-heavy and
// evolve; unmapped fields must be ignored, not fail the decode.
func TestAPICallDecodesNonStrictly(t *testing.T) {
	svc := newErrorService(t, `{"ok":true,"ts":"1512085950.000216","text":"hi","blocks":[{"type":"section","future_field":true}],"future_top_level":{"nested":1}}`)

	var out struct {
		TS   string `json:"ts"`
		Text string `json:"text"`
	}
	require.NoError(t, svc.apiCall(context.Background(), "/conversations.history", nil, &out))
	assert.Equal(t, "1512085950.000216", out.TS)
	assert.Equal(t, "hi", out.Text)
}

// TestAPICallQueryParams: query values are encoded into the request URL.
func TestAPICallQueryParams(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)
	svc := newHTTPService(srv.Client(), srv.URL)

	q := url.Values{}
	q.Set("channel", "C0B3HMXFEUV")
	q.Set("limit", "2")
	require.NoError(t, svc.apiCall(context.Background(), "/conversations.history", q, nil))
	assert.Equal(t, "channel=C0B3HMXFEUV&limit=2", got)
}

// TestAPICallNon200: a non-200, non-429 status becomes a descriptive error
// carrying the status, not an ok:false parse.
func TestAPICallNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusBadGateway)
	}))
	t.Cleanup(srv.Close)
	svc := newHTTPService(srv.Client(), srv.URL)

	err := svc.apiCall(context.Background(), "/auth.test", nil, &struct{}{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "502")
}

// TestAPICallNon200StripsControlBytes: the echoed error body is
// attacker-influenceable wire data, so checkStatus strips terminal control
// bytes (C0/DEL) from it while keeping the readable text.
func TestAPICallNon200StripsControlBytes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("fail\x1b[31mred\x07"))
	}))
	t.Cleanup(srv.Close)
	svc := newHTTPService(srv.Client(), srv.URL)

	err := svc.apiCall(context.Background(), "/auth.test", nil, &struct{}{})
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "\x1b")
	assert.NotContains(t, err.Error(), "\x07")
	assert.Contains(t, err.Error(), "fail")
	assert.Contains(t, err.Error(), "red")
}
