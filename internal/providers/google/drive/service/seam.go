package service

import (
	"context"
	"fmt"
)

// Dialer builds the service a leaf's RunE uses: the command parent injects
// the real dialer; tests inject fakes so no leaf ever touches the network.
// The type parameter is the per-concern interface the subtree consumes — the
// concrete service implements them all, and As narrows the seam.
type Dialer[T any] func(context.Context) (T, error)

// As adapts the DriveService a dialer returns into any of the per-concern
// interfaces (FileService, PermissionService, ...). The real service
// implements every drive interface; the assertion hands a subtree its own
// surface without growing the others. err is passed through untouched so the
// adapter composes directly on a dial call.
//
// svc is any, not DriveService: the drive, docs, sheets, and slides trees
// share this package, and each tree's service type must be adaptable through
// the same helper — typing svc to DriveService would force those services to
// satisfy drive's interface too.
func As[T any](svc any, err error) (T, error) {
	if err != nil {
		var zero T
		return zero, err
	}
	narrowed, ok := svc.(T)
	if !ok {
		return narrowed, fmt.Errorf("drive service does not implement the requested operations")
	}
	return narrowed, nil
}
