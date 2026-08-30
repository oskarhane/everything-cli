package service

import (
	"context"
	"fmt"
)

// Dialer builds the service a leaf's RunE uses: the gmail parent injects the
// real dialer; tests inject fakes so no leaf ever touches the network. The
// type parameter is the per-concern interface the subtree consumes — the
// concrete service implements them all, and As narrows the seam.
type Dialer[T any] func(context.Context) (T, error)

// As adapts the GmailService a dialer returns into any of the per-concern
// interfaces (MessageService, ThreadService, ...). The real service
// implements every gmail interface; the assertion hands a subtree its own
// surface without growing the others. err is passed through untouched so the
// adapter composes directly on a dial call.
func As[T any](svc GmailService, err error) (T, error) {
	if err != nil {
		var zero T
		return zero, err
	}
	narrowed, ok := svc.(T)
	if !ok {
		return narrowed, fmt.Errorf("gmail service does not implement the requested operations")
	}
	return narrowed, nil
}
