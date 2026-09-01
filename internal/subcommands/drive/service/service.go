// Package service is the seam between the drive command leaves and the
// Google Drive API. Leaves depend on the per-concern interfaces
// (FileService, PermissionService) so tests can run hermetically against a
// fake; only this package talks to the real API.
//
// The drive, docs, sheets, and slides trees share this package: each subtree
// adds its own interface in its own file here rather than growing an
// existing one, and As narrows the concrete service to whatever surface the
// subtree needs.
package service

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/oauth2"
	drive "google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

// DriveService is the full Drive API surface this package wraps. It is the
// type New returns and the type the Dialer injects; subtrees consume the
// narrower per-concern interfaces via As.
type DriveService interface {
	FileService
	PermissionService
}

// apiTimeout bounds each Drive API call so a hung endpoint cannot stall a
// command indefinitely.
const apiTimeout = 120 * time.Second

// New returns a DriveService bound to ts.
func New(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
	svc, err := drive.NewService(ctx, option.WithHTTPClient(authenticatedClient(ts, apiTimeout)))
	if err != nil {
		return nil, fmt.Errorf("creating drive service: %w", err)
	}
	return &realDriveService{drive: svc}, nil
}

// authenticatedClient builds the HTTP client every Drive call rides on.
// WithHTTPClient takes precedence over WithTokenSource (see option docs), so
// the token source must live inside the transport — a bare client here would
// send every request unauthenticated. oauth2.Transport adds the Bearer
// header; the client itself carries the timeout.
func authenticatedClient(ts oauth2.TokenSource, timeout time.Duration) *http.Client {
	return &http.Client{
		Transport: &oauth2.Transport{Base: http.DefaultTransport, Source: ts},
		Timeout:   timeout,
	}
}

// realDriveService adapts *drive.Service to DriveService.
type realDriveService struct {
	drive *drive.Service
}
