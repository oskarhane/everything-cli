package slack

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// defaultBaseURL is the Slack Web API origin. Every method is a path under
// it: /auth.test, /conversations.list, /search.messages, ...
const defaultBaseURL = "https://slack.com/api"

// apiTimeout bounds each Slack Web API call so a hung endpoint cannot stall a
// command indefinitely.
const apiTimeout = 60 * time.Second

// maxErrBodyBytes caps how many bytes of a non-200 response body are read
// and echoed into the error string, so a hostile or broken endpoint cannot
// exhaust memory or flood the terminal.
const maxErrBodyBytes = 4096

// apiEnvelope is the ok/error header every Slack Web API response carries,
// including HTTP-200 failures. Slack also reports needed/provided scopes on
// missing_scope.
type apiEnvelope struct {
	OK       bool   `json:"ok"`
	Error    string `json:"error"`
	Needed   string `json:"needed"`
	Provided string `json:"provided"`
}

// apiError is an application-level Slack failure: HTTP 200 with "ok": false.
// It carries the raw Slack error code (missing_scope, not_allowed_token_type,
// channel_not_found, ...) so callers and error messages can branch on it.
type apiError struct {
	Code     string
	Needed   string
	Provided string
}

// Error renders the code first, then the scope detail when Slack supplies
// it, so the message reads clearly in a one-line error output.
func (e *apiError) Error() string {
	msg := "slack API error: " + e.Code
	switch {
	case e.Needed != "" && e.Provided != "":
		msg += fmt.Sprintf(" (needed: %s; provided: %s)", e.Needed, e.Provided)
	case e.Needed != "":
		msg += fmt.Sprintf(" (needed: %s)", e.Needed)
	case e.Provided != "":
		msg += fmt.Sprintf(" (provided: %s)", e.Provided)
	}
	return msg
}

// httpService is the production Slack Web API client: plain HTTP over the
// authenticated client the API-key strategy builds (the strategy owns the
// token header). Every resource method goes through apiCall, which owns
// decoding and error mapping.
type httpService struct {
	client  *http.Client
	baseURL string
}

// newHTTPService binds the service to an authenticated client and API base
// URL. Tests pass an httptest server's URL and client. A client without a
// timeout gets apiTimeout so a hung endpoint cannot stall a command.
func newHTTPService(client *http.Client, baseURL string) *httpService {
	c := *client
	if c.Timeout == 0 {
		c.Timeout = apiTimeout
	}
	return &httpService{client: &c, baseURL: strings.TrimRight(baseURL, "/")}
}

// apiCall issues one authenticated GET against path with query params q and
// decodes the 200 body into out. Slack answers HTTP 200 with "ok": false for
// application errors; those become a typed apiError carrying the raw code.
// HTTP 429 surfaces the Retry-After seconds and is not retried. Decoding is
// deliberately non-strict (plain json.Unmarshal): Slack payloads are
// subtype-heavy and evolve, so only the pinned subset is mapped.
func (s *httpService) apiCall(ctx context.Context, path string, q url.Values, out any) error {
	u := s.baseURL + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("building slack request: %w", err)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("calling slack API: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusTooManyRequests {
		return rateLimitError(resp.Header.Get("Retry-After"))
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrBodyBytes))
		return fmt.Errorf("slack API returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading slack %s response: %w", path, err)
	}
	var env apiEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return fmt.Errorf("decoding slack %s response (upstream schema changed?): %w", path, err)
	}
	if !env.OK {
		code := env.Error
		if code == "" {
			code = "unknown_error"
		}
		return &apiError{Code: code, Needed: env.Needed, Provided: env.Provided}
	}
	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("decoding slack %s response (upstream schema changed?): %w", path, err)
		}
	}
	return nil
}

// rateLimitError renders Slack's 429 as an actionable error carrying the
// Retry-After seconds. There is no retry machinery: the caller decides
// whether to wait and re-run.
func rateLimitError(retryAfter string) error {
	retryAfter = strings.TrimSpace(retryAfter)
	if retryAfter == "" {
		return errors.New("slack API rate limit exceeded (429): retry shortly")
	}
	if secs, err := strconv.Atoi(retryAfter); err == nil {
		return fmt.Errorf("slack API rate limit exceeded (429): retry after %ds", secs)
	}
	return fmt.Errorf("slack API rate limit exceeded (429): retry after %s", retryAfter)
}
