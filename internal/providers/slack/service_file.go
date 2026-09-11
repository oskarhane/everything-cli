package slack

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// validateFileURL is the url_private allowlist: only https on slack.com, a
// .slack.com subdomain, or a .slack-edge.com subdomain may be fetched. It
// gates the download before any request because the bearer-stamping client
// would otherwise send the user's token to whatever absolute URL files.info
// returned. It is a var (like validateBaseURL) so hermetic tests can permit
// the httptest server host.
var validateFileURL = func(u *url.URL) bool {
	if u.Scheme != "https" {
		return false
	}
	host := u.Hostname()
	return host == "slack.com" ||
		strings.HasSuffix(host, ".slack.com") ||
		strings.HasSuffix(host, ".slack-edge.com")
}

// filesInfoResponse is the pinned subset of GET /files.info: the one file
// object under "file".
type filesInfoResponse struct {
	File wireFileInfo `json:"file"`
}

// wireFileInfo is the pinned subset of Slack's wire file object for
// files.info: the attachment fields wireFile already pins, plus
// URLPrivate, the absolute, token-gated download URL; it is wire-only —
// DownloadFileTo consumes it and it never reaches output.
type wireFileInfo struct {
	wireFile
	URLPrivate string `json:"url_private"`
}

// view maps the wire file into the shared File view, dropping url_private.
func (f wireFileInfo) view() File {
	return File(f.wireFile)
}

// FileInfo returns the file with the given Slack file ID (files.info). Only
// the shared File view is surfaced; url_private stays internal to the
// service and is never printed.
func (s *httpService) FileInfo(ctx context.Context, fileID string) (File, error) {
	info, err := s.fileInfoWire(ctx, fileID)
	if err != nil {
		return File{}, err
	}
	return info.view(), nil
}

// fileInfoWire fetches and returns the full wire file object, including the
// url_private download URL that FileInfo deliberately hides.
func (s *httpService) fileInfoWire(ctx context.Context, fileID string) (wireFileInfo, error) {
	q := url.Values{}
	q.Set("file", fileID)
	var out filesInfoResponse
	if err := s.apiCall(ctx, "/files.info", q, &out); err != nil {
		return wireFileInfo{}, err
	}
	return out.File, nil
}

// DownloadFileTo streams the file's bytes into w. files.info returns an
// absolute url_private outside baseURL, so this bypasses apiCall: it GETs that
// URL with the same authenticated client (the strategy stamps Authorization:
// Bearer), then copies the body. url_private is attacker-influenceable wire
// data, so it is validated against validateFileURL before any request and
// redirects off the original host are refused — Go's client strips
// Authorization on cross-domain redirects, but the strategy's transport
// re-stamps it on every RoundTrip. The download is unbounded — Drive parity,
// no size cap; the caller decides how much to consume. A 429 maps to the
// shared rate-limit error and any other non-200 to the status plus a body
// capped at maxErrBodyBytes.
func (s *httpService) DownloadFileTo(ctx context.Context, fileID string, w io.Writer) error {
	info, err := s.fileInfoWire(ctx, fileID)
	if err != nil {
		return err
	}
	if info.URLPrivate == "" {
		return fmt.Errorf("slack file %s has no download URL", fileID)
	}
	dlURL, err := url.Parse(info.URLPrivate)
	if err != nil || !validateFileURL(dlURL) {
		// Name the file id, never the URL: url_private must not leak into
		// output even when it is the reason for the failure.
		return fmt.Errorf("slack file %s has an untrusted download URL", fileID)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, dlURL.String(), nil)
	if err != nil {
		return fmt.Errorf("building slack file download request: %w", err)
	}
	// The bearer-stamping transport re-adds Authorization after Go strips it
	// on cross-domain redirects, so refuse to follow a redirect off the
	// download's original host rather than trusting the transport.
	client := *s.client
	client.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		if next.URL.Host != via[0].URL.Host {
			return errors.New("slack file download refused cross-host redirect")
		}
		return nil
	}
	resp, err := client.Do(req)
	if err != nil {
		// url.Error's message embeds the request URL; unwrap to the cause so
		// a network failure never echoes url_private.
		var uerr *url.Error
		if errors.As(err, &uerr) {
			err = uerr.Err
		}
		return fmt.Errorf("calling slack file download: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if err := s.checkStatus(resp); err != nil {
		return err
	}
	if _, err := io.Copy(w, resp.Body); err != nil {
		return fmt.Errorf("streaming slack file %s: %w", fileID, err)
	}
	return nil
}
