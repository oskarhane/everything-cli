package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// uploadsHost is the only host a linear file download may fetch: Linear's
// file-storage origin, where issue image uploads live. URL paths there are
// bare UUIDs — no filename or extension; the original filename arrives on
// Content-Disposition.
const uploadsHost = "uploads.linear.app"

// validateFileURL is the linear file download allowlist: only https on
// uploads.linear.app may be fetched, checked before any request — the
// api-key client stamps the account's Authorization header on every URL it
// GETs, which must never be sent to another origin. It is a var (like the
// slack seam's validateFileURL) so hermetic tests can permit the httptest
// server host.
var validateFileURL = func(u *url.URL) bool {
	return u.Scheme == "https" && u.Hostname() == uploadsHost
}

// FileService is the file-storage surface the `linear file` subtree consumes.
type FileService interface {
	// DownloadFileTo streams the bytes at one Linear file-storage URL into w.
	DownloadFileTo(ctx context.Context, fileURL string, w io.Writer) error
}

// Compile-time proof that Service satisfies the file surface.
var _ FileService = (*Service)(nil)

// DownloadFileTo streams the bytes at fileURL (a Linear file-storage URL,
// https://uploads.linear.app/<uuid>) into w through the authenticated
// client, so private files download with the acting account's credential
// exactly like the GraphQL calls. fileURL is validated against the allowlist
// before any request, and a redirect to another host is refused, because
// the transport re-stamps Authorization on every request — Go's redirect
// logic would otherwise strip it. A non-200 surfaces like every other
// linear call (status plus a capped body), the body streams (it is never
// materialized; the caller decides how much to consume), and a network
// failure surfaces its cause without echoing the URL, which may carry a
// signature query param on wire-resolved URLs.
func (s *Service) DownloadFileTo(ctx context.Context, fileURL string, w io.Writer) error {
	u, err := url.Parse(fileURL)
	if err != nil || !validateFileURL(u) {
		return fmt.Errorf("linear file download refused URL: only https://%s is allowed", uploadsHost)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return fmt.Errorf("building linear file download request: %w", err)
	}
	client := *s.client
	client.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		if next.URL.Host != via[0].URL.Host {
			return errors.New("linear file download refused cross-host redirect")
		}
		return nil
	}
	resp, err := client.Do(req)
	if err != nil {
		// url.Error's message embeds the request URL; unwrap to the cause so a
		// network failure never echoes it.
		var uerr *url.Error
		if errors.As(err, &uerr) {
			err = uerr.Err
		}
		return fmt.Errorf("calling linear file download: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		// Read one byte past the cap so truncation is detected and marked.
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrBodyBytes+1))
		return fmt.Errorf("linear API returned %s: %s", resp.Status, truncateBody(respBody))
	}
	if _, err := io.Copy(w, resp.Body); err != nil {
		return fmt.Errorf("streaming linear file download: %w", err)
	}
	return nil
}
