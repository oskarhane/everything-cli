package service

import (
	"context"
	"errors"
	"testing"
)

// testSurface is a fake per-concern interface: a type implementing it can be
// narrowed via As only when the assertion succeeds.
type testSurface interface{ Surface() string }

type fakeFullService struct{}

func (fakeFullService) Surface() string { return "full" }

type fakeWrongService struct{}

func (fakeWrongService) Other() string { return "other" }

func TestAs_NarrowsToTheRequestedSurface(t *testing.T) {
	got, err := As[testSurface](fakeFullService{}, nil)
	if err != nil {
		t.Fatalf("As: %v", err)
	}
	if got.Surface() != "full" {
		t.Fatalf("surface = %q, want %q", got.Surface(), "full")
	}
}

func TestAs_PassesDialErrorThroughUntouched(t *testing.T) {
	// As composes directly on a dial call: the dialer's error must come back
	// as-is, not be shadowed by the narrowing logic.
	sentinel := errors.New("dial failed")
	got, err := As[testSurface](fakeFullService{}, sentinel)
	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want the dial error passed through", err)
	}
	if got != nil {
		t.Fatalf("narrowed = %v, want the zero value on dial error", got)
	}
}

func TestAs_ErrorsOnWrongOperations(t *testing.T) {
	// A service that does not implement the requested surface must fail with
	// the drive-specific message.
	_, err := As[testSurface](fakeWrongService{}, nil)
	if err == nil {
		t.Fatal("As: want error for a service without the requested operations, got nil")
	}
	if err.Error() != "drive service does not implement the requested operations" {
		t.Fatalf("err = %q, want the drive seam error text", err.Error())
	}
}

func TestDialerBuildsItsService(t *testing.T) {
	// Dialer is the seam the parent injects and leaves call; pin that a
	// dialer composes straight into As.
	var dialer Dialer[testSurface] = func(context.Context) (testSurface, error) {
		return fakeFullService{}, nil
	}
	got, err := As[testSurface](dialer(t.Context()))
	if err != nil {
		t.Fatalf("As on dialer result: %v", err)
	}
	if got.Surface() != "full" {
		t.Fatalf("surface = %q, want %q", got.Surface(), "full")
	}
}
