package provider

import (
	"fmt"
	"sort"
	"sync"
)

// registry is the process-wide provider set. Registration happens from
// provider packages' init() functions, wired in via side-effect imports in
// main.go (chosen over explicit calls in main so adding a provider is one
// import line and cannot drift out of sync with the tree). A global is
// acceptable here because the registry is written only at init time —
// before main runs — and read afterwards.
var (
	registryMu sync.RWMutex
	registry   = map[string]Provider{}
)

// Register adds p to the registry under p.ID(). It panics on a duplicate
// ID: that is a programmer error (two providers claiming the same key)
// that must fail loudly at startup, not a runtime condition to handle.
func Register(p Provider) {
	registryMu.Lock()
	defer registryMu.Unlock()
	id := p.ID()
	if id == "" {
		panic("provider: Register called with empty ID")
	}
	if _, exists := registry[id]; exists {
		panic(fmt.Sprintf("provider: duplicate registration for ID %q", id))
	}
	registry[id] = p
}

// Get returns the provider registered under id, or false if none is.
func Get(id string) (Provider, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	p, ok := registry[id]
	return p, ok
}

// List returns all registered providers sorted by ID, so help output and
// any iteration over providers is deterministic regardless of init order.
func List() []Provider {
	registryMu.RLock()
	defer registryMu.RUnlock()
	out := make([]Provider, 0, len(registry))
	for _, p := range registry {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID() < out[j].ID() })
	return out
}
