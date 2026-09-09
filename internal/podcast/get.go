package podcast

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// httpClient is the package's single outbound GET pipeline, driving every
// metadata (Apple episode page, iTunes lookup/search, Spotify embed page),
// feed, and transcript fetch. The 30s timeout bounds each call so a hung
// endpoint cannot stall resolution indefinitely. Redirects are followed (the
// Apple episode page answers a 301 on first hit), but every hop target is
// re-validated against the https-only (or loopback test server) rule, so a
// redirect cannot slip the client onto an arbitrary plain-HTTP origin.
var httpClient = &http.Client{
	Timeout: 30 * time.Second,
	CheckRedirect: func(req *http.Request, _ []*http.Request) error {
		if err := checkHTTPS(req.URL); err != nil {
			return fmt.Errorf("podcast: refusing redirect to %q: %w", req.URL.Redacted(), err)
		}
		return nil
	},
}

// maxBodyBytes caps how large any fetched body (HTML page, iTunes JSON, RSS
// feed, or transcript) may be before the fetch errors, so a misbehaving or
// hostile server cannot balloon memory.
const maxBodyBytes = 64 << 20 // 64 MiB

// get fetches rawURL over GET, following redirects (each hop re-validated by
// httpClient.CheckRedirect), and returns the 200 body capped at maxBodyBytes;
// a body larger than the cap is an error, never a silent truncation.
// userAgent is set verbatim when non-empty (only the Apple episode page needs
// a browser User-Agent); when empty no explicit UA header is set. Non-200
// statuses become an error and the https-only guard rejects any initial
// target not served over https or a loopback test server.
func get(ctx context.Context, rawURL, userAgent string) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("podcast: invalid URL %q: %w", rawURL, err)
	}
	if err := checkHTTPS(u); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("podcast: building request: %w", err)
	}
	if userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("podcast: fetching %s: %w", u.Host, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("podcast: unexpected HTTP status %d from %s", resp.StatusCode, u.Host)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes+1))
	if err != nil {
		return nil, fmt.Errorf("podcast: reading %s: %w", u.Host, err)
	}
	if int64(len(body)) > maxBodyBytes {
		return nil, fmt.Errorf("podcast: response from %s exceeds %d bytes", u.Host, maxBodyBytes)
	}
	return body, nil
}

// checkHTTPS rejects any target not served over https. Loopback hosts (used
// by httptest test servers) are the only plain-HTTP exception.
func checkHTTPS(u *url.URL) error {
	if u.Scheme == "https" {
		return nil
	}
	switch u.Hostname() {
	case "localhost", "127.0.0.1", "::1":
		return nil
	}
	return fmt.Errorf("podcast: refusing non-https URL %q", u.Redacted())
}
