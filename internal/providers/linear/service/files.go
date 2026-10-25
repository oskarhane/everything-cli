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

// FileMeta reports what a completed download learned about the fetched
// file: the response's Content-Type — a storage URL's path is a bare UUID
// with no filename or extension, so callers that write a local file name
// it from the response — and the number of body bytes streamed.
type FileMeta struct {
	ContentType string
	Bytes       int64
}

// FileService is the file-storage surface the `linear file` and `linear
// issue image` subtrees consume.
type FileService interface {
	// DownloadFileTo streams the bytes at one Linear file-storage URL into w.
	DownloadFileTo(ctx context.Context, fileURL string, w io.Writer) error
	// DownloadFileMeta streams exactly like DownloadFileTo and additionally
	// reports the response's Content-Type and byte count, so a caller can
	// name its destination from the response (e.g. fix a missing filename
	// extension from the Content-Type).
	DownloadFileMeta(ctx context.Context, fileURL string, w io.Writer) (FileMeta, error)
}

// Compile-time proof that Service satisfies the file surface.
var _ FileService = (*Service)(nil)

// DownloadFileTo streams the bytes at fileURL (a Linear file-storage URL,
// https://uploads.linear.app/<uuid>) into w; DownloadFileMeta is the full
// implementation, whose doc comment covers the guard and error behavior.
func (s *Service) DownloadFileTo(ctx context.Context, fileURL string, w io.Writer) error {
	_, err := s.DownloadFileMeta(ctx, fileURL, w)
	return err
}

// DownloadFileMeta streams the bytes at fileURL (a Linear file-storage
// URL, https://uploads.linear.app/<uuid>) into w through the authenticated
// client, so private files download with the acting account's credential
// exactly like the GraphQL calls, and reports what the response said
// about the file: its Content-Type and its body byte count. fileURL is
// validated against the allowlist before any request, and a redirect that
// changes the scheme — an https to http downgrade on the same host included
// — or targets another host is refused, because the transport re-stamps
// Authorization on every request — Go's redirect logic would otherwise
// strip it, and a downgrade would put it on the wire in the clear. A
// non-200 surfaces like every other linear call (status plus a capped
// body), the body streams (it is never materialized; the caller decides
// how much to consume), and a network failure surfaces its cause without
// echoing the URL, which may carry a signature query param on
// wire-resolved URLs.
func (s *Service) DownloadFileMeta(ctx context.Context, fileURL string, w io.Writer) (FileMeta, error) {
	u, err := url.Parse(fileURL)
	if err != nil || !validateFileURL(u) {
		return FileMeta{}, fmt.Errorf("linear file download refused URL: only https://%s is allowed", uploadsHost)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return FileMeta{}, fmt.Errorf("building linear file download request: %w", err)
	}
	client := *s.client
	client.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		if next.URL.Scheme != via[0].URL.Scheme {
			return errors.New("linear file download refused redirect scheme change")
		}
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
		return FileMeta{}, fmt.Errorf("calling linear file download: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		// Read one byte past the cap so truncation is detected and marked.
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrBodyBytes+1))
		return FileMeta{}, fmt.Errorf("linear API returned %s: %s", resp.Status, truncateBody(respBody))
	}
	n, err := io.Copy(w, resp.Body)
	if err != nil {
		return FileMeta{}, fmt.Errorf("streaming linear file download: %w", err)
	}
	return FileMeta{ContentType: resp.Header.Get("Content-Type"), Bytes: n}, nil
}
