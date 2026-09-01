package provider

import (
	"sort"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/auth"
)

// fakeProvider is the minimal stub the registry tests need: NewCmd and Auth
// are never exercised, so they return placeholders.
type fakeProvider struct{ id string }

func (f fakeProvider) ID() string                          { return f.id }
func (f fakeProvider) NewCmd(_ *app.Config) *cobra.Command { return &cobra.Command{Use: f.id} }
func (f fakeProvider) Auth() auth.Strategy                 { return nil }

// Compile-time proof the stub satisfies the contract under test.
var _ Provider = fakeProvider{}

func TestRegisterAndGet(t *testing.T) {
	p := fakeProvider{id: "fake-get"}
	Register(p)

	got, ok := Get("fake-get")
	require.True(t, ok, "registered provider must be retrievable by ID")
	assert.Equal(t, "fake-get", got.ID())

	_, ok = Get("fake-missing")
	assert.False(t, ok, "unregistered ID must return ok=false")
}

func TestListSortedDeterministic(t *testing.T) {
	// Register out of order; List must come back sorted by ID regardless
	// of registration (init) order.
	Register(fakeProvider{id: "fake-zulu"})
	Register(fakeProvider{id: "fake-alpha"})
	Register(fakeProvider{id: "fake-mike"})

	ids := make([]string, 0, len(List()))
	for _, p := range List() {
		ids = append(ids, p.ID())
	}
	assert.True(t, sort.IsSorted(sort.StringSlice(ids)), "List must be sorted by ID, got %v", ids)

	// The three fakes of this test appear in relative sorted order.
	assert.Less(t, indexOf(ids, "fake-alpha"), indexOf(ids, "fake-mike"))
	assert.Less(t, indexOf(ids, "fake-mike"), indexOf(ids, "fake-zulu"))
}

func TestRegisterDuplicatePanics(t *testing.T) {
	Register(fakeProvider{id: "fake-dupe"})
	assert.Panics(t, func() { Register(fakeProvider{id: "fake-dupe"}) },
		"duplicate ID is a programmer error and must fail loudly")
}

func TestRegisterEmptyIDPanics(t *testing.T) {
	assert.Panics(t, func() { Register(fakeProvider{id: ""}) },
		"an empty ID would be an unaddressable registry entry")
}

func indexOf(ids []string, want string) int {
	for i, id := range ids {
		if id == want {
			return i
		}
	}
	return -1
}
