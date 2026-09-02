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
	docs "google.golang.org/api/docs/v1"
	drive "google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
	sheets "google.golang.org/api/sheets/v4"
	slides "google.golang.org/api/slides/v1"
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

// New returns a DriveService bound to ts. Besides the drive client it also
// builds the docs, sheets, and slides clients on the same authenticated HTTP
// client, so the whole Google Workspace surface shares one transport (one
// token source, one timeout).
func New(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
	client := authenticatedClient(ts, apiTimeout)
	svc, err := drive.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("creating drive service: %w", err)
	}
	docSvc, err := docs.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("creating docs service: %w", err)
	}
	sheetSvc, err := sheets.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("creating sheets service: %w", err)
	}
	slideSvc, err := slides.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("creating slides service: %w", err)
	}
	return &realDriveService{drive: svc, docs: docSvc, sheets: sheetSvc, slides: slideSvc}, nil
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

// realDriveService adapts the Drive, Docs, Sheets, and Slides API clients to
// DriveService (plus the per-concern interfaces each subtree's file defines).
// All four clients share the one authenticated HTTP client.
type realDriveService struct {
	drive  *drive.Service
	docs   *docs.Service
	sheets *sheets.Service
	slides *slides.Service
}
