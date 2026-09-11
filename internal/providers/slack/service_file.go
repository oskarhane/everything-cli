package slack

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// filesInfoResponse is the pinned subset of GET /files.info: the one file
// object under "file".
type filesInfoResponse struct {
	File wireFileInfo `json:"file"`
}

// wireFileInfo is the pinned subset of Slack's wire file object for
// files.info. URLPrivate is the absolute, token-gated download URL; it is
// wire-only — DownloadFileTo consumes it and it never reaches output.
type wireFileInfo struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Mimetype   string `json:"mimetype"`
	Size       int64  `json:"size"`
	URLPrivate string `json:"url_private"`
}

// view maps the wire file into the shared File view, dropping url_private.
func (f wireFileInfo) view() File {
	return File{ID: f.ID, Name: f.Name, Mimetype: f.Mimetype, Size: f.Size}
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
// Bearer), then copies the body. The download is unbounded — Drive parity, no
// size cap; the caller decides how much to consume. A 429 maps to the shared
// rate-limit error and any other non-200 to the status plus a body capped at
// maxErrBodyBytes.
func (s *httpService) DownloadFileTo(ctx context.Context, fileID string, w io.Writer) error {
	info, err := s.fileInfoWire(ctx, fileID)
	if err != nil {
		return err
	}
	if info.URLPrivate == "" {
		return fmt.Errorf("slack file %s has no download URL", fileID)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, info.URLPrivate, nil)
	if err != nil {
		return fmt.Errorf("building slack file download request: %w", err)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("calling slack file download: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusTooManyRequests {
		return rateLimitError(resp.Header.Get("Retry-After"))
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrBodyBytes))
		return fmt.Errorf("slack API returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if _, err := io.Copy(w, resp.Body); err != nil {
		return fmt.Errorf("streaming slack file %s: %w", fileID, err)
	}
	return nil
}
