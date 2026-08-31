package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

const (
	defaultGitHubBase = "https://api.github.com"
	defaultRepo       = "oskarhane/google-cli"
	acceptHeader      = "application/vnd.github+json"

	// maxBodyBytes caps how many bytes are read from any single response
	// body. Release metadata responses are tiny (a few KB); the largest
	// legit release asset is a compressed Go binary, well under 16 MB, so
	// 256 MB leaves a ~16x margin while still bounding memory from a
	// compromised or hostile response instead of exhausting the process.
	maxBodyBytes = 256 << 20
)

// maxBodyLimit is a var seam over maxBodyBytes so tests can lower the cap.
var maxBodyLimit = int64(maxBodyBytes)

// Client fetches release metadata and asset payloads from GitHub.
type Client interface {
	// LatestRelease returns the latest published release.
	LatestRelease(ctx context.Context) (*Release, error)
	// Download fetches the bytes at an absolute asset URL.
	Download(ctx context.Context, url string) ([]byte, error)
}

// Release is a GitHub release with its downloadable assets.
type Release struct {
	Tag    string  `json:"tag_name"`
	Assets []Asset `json:"assets"`
}

// Asset is a downloadable release artifact.
type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

// Asset returns the release asset with the given name, or ErrAssetNotFound.
func (r *Release) Asset(name string) (*Asset, error) {
	for i := range r.Assets {
		if r.Assets[i].Name == name {
			return &r.Assets[i], nil
		}
	}
	return nil, fmt.Errorf("%w: %s", ErrAssetNotFound, name)
}

// assetName is the single source of truth for release asset naming.
func assetName(os, arch string) string {
	return fmt.Sprintf("google-cli_%s_%s.tar.gz", os, arch)
}

// HTTPClient is the Client implementation backed by the GitHub REST API.
type HTTPClient struct {
	base       string
	repo       string
	httpClient *http.Client
}

// NewClient builds a GitHub releases Client. Empty baseURL or repo fall
// back to https://api.github.com and oskarhane/google-cli.
func NewClient(baseURL, repo string) *HTTPClient {
	if baseURL == "" {
		baseURL = defaultGitHubBase
	}
	if repo == "" {
		repo = defaultRepo
	}
	return &HTTPClient{
		base:       baseURL,
		repo:       repo,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

// LatestRelease fetches the latest published release for the configured repo.
func (c *HTTPClient) LatestRelease(ctx context.Context) (*Release, error) {
	endpoint := c.base + "/repos/" + c.repo + "/releases/latest"
	body, err := c.get(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	var rel Release
	if err := json.Unmarshal(body, &rel); err != nil {
		return nil, fmt.Errorf("parsing release response: %w", err)
	}
	return &rel, nil
}

// Download fetches the bytes at an absolute asset URL.
func (c *HTTPClient) Download(ctx context.Context, assetURL string) ([]byte, error) {
	return c.get(ctx, assetURL)
}

func (c *HTTPClient) get(ctx context.Context, endpoint string) ([]byte, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid endpoint URL: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Accept", acceptHeader)
	if tok := authToken(); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("requesting %s: %w", u.Host, err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusOK:
		// Read one byte past the cap so an oversized body is detected as a
		// distinct error rather than silently returning a truncated body.
		data, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyLimit+1))
		if err != nil {
			return nil, fmt.Errorf("reading response from %s: %w", u.Host, err)
		}
		if int64(len(data)) > maxBodyLimit {
			return nil, fmt.Errorf("response from %s exceeds %d bytes", u.Host, maxBodyLimit)
		}
		return data, nil
	case http.StatusNotFound:
		return nil, ErrNoReleases
	case http.StatusForbidden:
		return nil, ErrRateLimited
	default:
		return nil, fmt.Errorf("unexpected status %s from %s", resp.Status, u.Host)
	}
}

// authToken reads the auth token from the environment. The value is used
// only for a request header — it never appears in error strings or debug
// output.
func authToken() string {
	for _, key := range []string{"GITHUB_TOKEN", "GH_TOKEN"} {
		if v := os.Getenv(key); v != "" {
			return v
		}
	}
	return ""
}
