// Package service is the seam between the linear command leaves and the
// Linear GraphQL API. Leaves depend on the per-concern interfaces
// (IssueService, TeamService, ProjectService) so tests can run
// hermetically against a fake; only this package talks to the real API.
// The single GraphQL HTTP call (exec) and Relay cursor pagination
// (collectPages) live here — no leaf ever dials or paginates itself.
package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Endpoint is the Linear GraphQL API endpoint: one URL for every operation.
const Endpoint = "https://api.linear.app/graphql"

// apiTimeout bounds each Linear API call so a hung endpoint cannot stall a
// command indefinitely.
const apiTimeout = 120 * time.Second

// maxErrBodyBytes caps how many bytes of a non-200 response body are read
// and echoed into the error string, so a hostile or broken endpoint cannot
// exhaust memory or flood the terminal.
const maxErrBodyBytes = 4096

// Service is the full Linear API surface this package wraps. It is the type
// New returns; subtrees consume the narrower per-concern interfaces via As.
type Service struct {
	client   *http.Client
	endpoint string
}

// Compile-time proof that Service satisfies every per-concern surface.
var (
	_ IssueService   = (*Service)(nil)
	_ TeamService    = (*Service)(nil)
	_ ProjectService = (*Service)(nil)
)

// New returns a Service bound to client, an authenticated HTTP client (the
// auth strategy's Client) carrying the API key on every request.
func New(client *http.Client) *Service {
	return NewForEndpoint(client, Endpoint)
}

// NewForEndpoint is New with an explicit endpoint, so tests can point the
// real service at an httptest server.
func NewForEndpoint(client *http.Client, endpoint string) *Service {
	c := *client
	if c.Timeout == 0 {
		c.Timeout = apiTimeout
	}
	return &Service{client: &c, endpoint: endpoint}
}

// gqlRequest is the POST body of every Linear API call.
type gqlRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

// gqlError is one entry of the GraphQL errors array.
type gqlError struct {
	Message    string `json:"message"`
	Extensions struct {
		Code string `json:"code"`
	} `json:"extensions"`
}

// exec posts one GraphQL operation and returns the raw data document. It is
// the only place an HTTP call to Linear happens. A non-200 status or a
// non-empty errors array is an error; Linear reports rate limiting as HTTP
// 400 with a RATELIMITED extension code, which surfaces through the errors
// branch.
func (s *Service) exec(ctx context.Context, query string, variables map[string]any) (json.RawMessage, error) {
	body, err := json.Marshal(gqlRequest{Query: query, Variables: variables})
	if err != nil {
		return nil, fmt.Errorf("encoding graphql request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("building graphql request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling linear API: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		// Read one byte past the cap so truncation is detected and marked.
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrBodyBytes+1))
		return nil, fmt.Errorf("linear API returned %s: %s", resp.Status, truncateBody(respBody))
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading linear response: %w", err)
	}
	var envelope struct {
		Data   json.RawMessage `json:"data"`
		Errors []gqlError      `json:"errors"`
	}
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return nil, fmt.Errorf("decoding linear response: %w", err)
	}
	if len(envelope.Errors) > 0 {
		msgs := make([]string, 0, len(envelope.Errors))
		for _, e := range envelope.Errors {
			if e.Extensions.Code != "" {
				msgs = append(msgs, fmt.Sprintf("%s (%s)", e.Message, e.Extensions.Code))
				continue
			}
			msgs = append(msgs, e.Message)
		}
		return nil, fmt.Errorf("linear API error: %s", strings.Join(msgs, "; "))
	}
	return envelope.Data, nil
}

// truncateBody renders a non-200 response body for an error string, capped
// at maxErrBodyBytes with an ellipsis marking the cut.
func truncateBody(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > maxErrBodyBytes {
		return s[:maxErrBodyBytes] + "..."
	}
	return s
}

// mutationPayload runs one mutation and returns its created/updated node,
// failing when the payload reports success: false or carries a null node.
// Every Linear mutation payload is { success, <field> } under the mutation
// key; dig locates both halves without per-mutation payload structs.
func mutationPayload[T any](ctx context.Context, s *Service, query string, variables map[string]any, key, field string) (*T, error) {
	data, err := s.exec(ctx, query, variables)
	if err != nil {
		return nil, err
	}
	rawSuccess, err := dig(data, key, "success")
	if err != nil {
		return nil, err
	}
	var success bool
	if err := json.Unmarshal(rawSuccess, &success); err != nil {
		return nil, fmt.Errorf("decoding %s success: %w", key, err)
	}
	rawNode, err := dig(data, key, field)
	if err != nil {
		return nil, err
	}
	var node *T
	if err := json.Unmarshal(rawNode, &node); err != nil {
		return nil, fmt.Errorf("decoding %s %s: %w", key, field, err)
	}
	if !success || node == nil {
		return nil, fmt.Errorf("linear API reported %s success: false", key)
	}
	return node, nil
}

// dig walks data down the given key path and returns the raw JSON found
// there. It locates nested results such as issue.comments. A null at an
// intermediate step leaves nowhere left to walk and is an error; a null at
// the final step is returned as-is (GetIssue turns it into not-found).
func dig(data json.RawMessage, path ...string) (json.RawMessage, error) {
	raw := data
	for i, key := range path {
		var m map[string]json.RawMessage
		if err := json.Unmarshal(raw, &m); err != nil {
			return nil, fmt.Errorf("decoding linear response at %q: %w", key, err)
		}
		next, ok := m[key]
		if !ok {
			return nil, fmt.Errorf("linear response missing %q", key)
		}
		if string(next) == "null" && i < len(path)-1 {
			return nil, fmt.Errorf("linear response has null %q", key)
		}
		raw = next
	}
	return raw, nil
}
