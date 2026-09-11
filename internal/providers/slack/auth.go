package slack

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/oskarhane/everything-cli/internal/auth/apikey"
)

// authTestResponse is the pinned subset of GET /auth.test: the identity of
// the token's owner and workspace. Unmapped fields (url, bot_id, ...) are
// deliberately ignored.
type authTestResponse struct {
	Team   string `json:"team"`
	TeamID string `json:"team_id"`
	User   string `json:"user"`
	UserID string `json:"user_id"`
}

// validateBaseURL is the origin the validate hook probes. It is a var so the
// account tests can point the probe at an httptest server.
var validateBaseURL = defaultBaseURL

// warnWriter receives the validate hook's non-fatal warnings. It is a var
// (default os.Stderr) so tests capture warnings without reading process
// stderr.
var warnWriter io.Writer = os.Stderr

// validateKey is the hook apikey runs before persisting an account: it
// probes auth.test for the token's workspace identity and warns when the
// token is not a user token — only xoxp- tokens can call search.messages.
// It is a var so tests can stub the probe instead of reaching the network.
var validateKey = func(ctx context.Context, key string) (map[string]string, error) {
	if !strings.HasPrefix(key, "xoxp-") {
		_, _ = fmt.Fprintf(warnWriter, "warning: the Slack API key does not start with xoxp-; search will not work with a bot or app token\n")
	}
	svc := newHTTPService(keyClient(key), validateBaseURL)
	var out authTestResponse
	if err := svc.apiCall(ctx, "/auth.test", nil, &out); err != nil {
		return nil, err
	}
	return map[string]string{
		"team":    out.Team,
		"team_id": out.TeamID,
		"user":    out.User,
		"user_id": out.UserID,
	}, nil
}

// strategy is Slack's auth strategy: an xoxp- user token sent as
// "Authorization: Bearer <token>", captured from --api-key, then
// SLACK_API_KEY, then a hidden prompt. The token is a secret on par with an
// OAuth refresh token: apikey registers it for redaction at capture and read
// points, and no command ever prints it. Validate runs auth.test before the
// account is persisted and stores the workspace identity. The config is
// compile-time constant, so Must panics on a misconfiguration rather than
// returning an error no caller could handle.
var strategy = apikey.Must(apikey.Config{
	Provider:     providerID,
	HeaderName:   "Authorization",
	HeaderFormat: "Bearer %s",
	EnvVar:       "SLACK_API_KEY",
	Validate: func(ctx context.Context, key string) (map[string]string, error) {
		return validateKey(ctx, key)
	},
})

// keyClient returns a client that stamps the raw key onto every request as
// "Authorization: Bearer <key>". The validate hook runs before an account
// exists, so it cannot use strategy.Client; once an account is stored, the
// API-key strategy owns the token header instead.
func keyClient(key string) *http.Client {
	return &http.Client{Transport: &bearerTransport{token: key, base: http.DefaultTransport}}
}

// bearerTransport sets the Bearer authorization on every request, cloning
// the request so the caller's is never mutated.
type bearerTransport struct {
	token string
	base  http.RoundTripper
}

func (t *bearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r := req.Clone(req.Context())
	r.Header.Set("Authorization", "Bearer "+t.token)
	return t.base.RoundTrip(r)
}
