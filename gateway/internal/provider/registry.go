package provider

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Factory builds a Provider instance. Adapters typically pass a no-arg
// constructor (e.g. openai.New) wrapped to return the interface type.
type Factory func() (Provider, error)

var (
	registryMu sync.RWMutex
	registry   = map[string]Factory{}
)

// Register adds a provider factory to the package-level registry. Adapter
// packages call this from their init() function:
//
//	func init() {
//	    provider.Register("openai", func() (provider.Provider, error) {
//	        return New(), nil
//	    })
//	}
//
// Register panics on duplicate registration to make accidental name
// collisions visible at startup rather than at request time.
func Register(name string, f Factory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	if name == "" {
		panic("provider: empty name")
	}
	if f == nil {
		panic("provider: nil factory for " + name)
	}
	if _, exists := registry[name]; exists {
		panic("provider: duplicate registration for " + name)
	}
	registry[name] = f
}

// Lookup constructs the provider instance for the given name, or returns
// an error if no adapter is registered.
func Lookup(name string) (Provider, error) {
	registryMu.RLock()
	f, ok := registry[name]
	registryMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("provider: no adapter registered for %q", name)
	}
	return f()
}

// Names returns the registered provider names, sorted, for diagnostics.
func Names() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	out := make([]string, 0, len(registry))
	for name := range registry {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// FromPath inspects a request path and returns the provider name encoded
// in the leading URL segment, or an error if the segment doesn't match a
// registered adapter.
//
// Examples:
//
//	/openai/v1/chat/completions      -> "openai"
//	/anthropic/v1/messages           -> "anthropic"
//	/google/v1beta/models/foo:bar    -> "google"
//	/azure/openai/deployments/x/...  -> "azure"
func FromPath(path string) (string, error) {
	trimmed := strings.TrimPrefix(path, "/")
	if trimmed == "" {
		return "", fmt.Errorf("provider: empty path")
	}
	idx := strings.IndexByte(trimmed, '/')
	if idx == -1 {
		return "", fmt.Errorf("provider: missing provider segment in %q", path)
	}
	name := trimmed[:idx]
	registryMu.RLock()
	_, ok := registry[name]
	registryMu.RUnlock()
	if !ok {
		return "", fmt.Errorf("provider: no adapter registered for %q (from path %q)", name, path)
	}
	return name, nil
}

// reset is used by tests to wipe the registry between cases.
func reset() {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry = map[string]Factory{}
}
