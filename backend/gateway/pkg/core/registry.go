package core

import (
	"fmt"
	"sort"
	"sync"

	"github.com/insmtx/Leros/backend/gateway/pkg/types"
)

// AdapterFactory creates a configured channel adapter from its config.
//
// The factory receives the raw channel config and any shared gateway
// dependencies the adapter needs (event bus publishers, DB handles, etc.).
type AdapterFactory func(cfg types.ChannelConfig) (any, error)

// EnabledFunc reports whether a channel should be registered given the
// gateway configuration. Returning false means the channel is skipped entirely
// at startup (no adapter is created, no routes are registered).
type EnabledFunc func() bool

// ChannelEntry describes one registered channel and its metadata.
//
// Each entry is registered at init time (for built-in channels) or at
// plugin load time (for external adapters). The registry uses this
// information to instantiate and manage channel lifecycles.
type ChannelEntry struct {
	// Code is the unique channel identifier (e.g., "feishu", "github").
	Code types.ChannelCode
	// Label is the human-readable display name.
	Label string
	// Description explains the channel’s purpose.
	Description string
	// Version is the adapter implementation version.
	Version string
	// Capabilities declares what the adapter can do.
	Capabilities types.ChannelCapabilities
	// Order controls the startup sequence. Lower numbers start first.
	Order int
	// Enabled determines whether this channel is active in the current config.
	Enabled EnabledFunc
	// Factory creates the adapter instance when the channel is enabled.
	Factory AdapterFactory
}

// Registry stores and manages channel adapter entries.
//
// It is safe for concurrent registration (e.g., plugins calling Register
// during init) and provides ordered iteration for deterministic startup.
type Registry struct {
	mu      sync.RWMutex
	entries map[types.ChannelCode]*ChannelEntry
}

// NewRegistry creates an empty channel registry.
func NewRegistry() *Registry {
	return &Registry{
		entries: make(map[types.ChannelCode]*ChannelEntry),
	}
}

// Register adds a channel entry to the registry.
//
// Returns an error if:
//   - Code is empty
//   - Factory is nil
//   - A channel with the same Code is already registered
func (r *Registry) Register(entry ChannelEntry) error {
	if entry.Code == "" {
		return fmt.Errorf("channel code is required")
	}
	if entry.Factory == nil {
		return fmt.Errorf("channel %s: factory is required", entry.Code)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.entries[entry.Code]; exists {
		return fmt.Errorf("channel %s: already registered", entry.Code)
	}

	r.entries[entry.Code] = &entry
	return nil
}

// MustRegister is like Register but panics on error. Use only during init()
// for built-in channels where duplicate registration is a programming error.
func (r *Registry) MustRegister(entry ChannelEntry) {
	if err := r.Register(entry); err != nil {
		panic(fmt.Sprintf("channel registry: %v", err))
	}
}

// Get returns the entry for a channel, or nil if not found.
func (r *Registry) Get(code types.ChannelCode) *ChannelEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.entries[code]
}

// List returns all registered entries sorted by Order, then by Code.
// Enabled channels are not filtered — callers should check Entry.Enabled().
func (r *Registry) List() []ChannelEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entries := make([]ChannelEntry, 0, len(r.entries))
	for _, e := range r.entries {
		entries = append(entries, *e)
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Order != entries[j].Order {
			return entries[i].Order < entries[j].Order
		}
		return entries[i].Code < entries[j].Code
	})

	return entries
}

// Enabled returns only the entries whose Enabled() function returns true,
// sorted by Order then Code.
func (r *Registry) Enabled() []ChannelEntry {
	all := r.List()
	enabled := make([]ChannelEntry, 0, len(all))
	for _, e := range all {
		if e.Enabled == nil || e.Enabled() {
			enabled = append(enabled, e)
		}
	}
	return enabled
}

// Len returns the total number of registered channels (enabled and disabled).
func (r *Registry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.entries)
}

// Codes returns the codes of all registered channels.
func (r *Registry) Codes() []types.ChannelCode {
	r.mu.RLock()
	defer r.mu.RUnlock()

	codes := make([]types.ChannelCode, 0, len(r.entries))
	for code := range r.entries {
		codes = append(codes, code)
	}
	sort.Slice(codes, func(i, j int) bool { return codes[i] < codes[j] })
	return codes
}
