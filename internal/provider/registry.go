package provider

import (
	"fmt"
	"sync"
)

// Registry holds named providers.
type Registry struct {
	mu        sync.RWMutex
	providers map[string]Provider
	defaultName string
}

func NewRegistry() *Registry {
	return &Registry{providers: make(map[string]Provider)}
}

func (r *Registry) Register(p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[p.Name()] = p
	if r.defaultName == "" {
		r.defaultName = p.Name()
	}
}

func (r *Registry) SetDefault(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.defaultName = name
}

func (r *Registry) Get(name string) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if name == "" {
		name = r.defaultName
	}
	p, ok := r.providers[name]
	if !ok || p == nil {
		return nil, fmt.Errorf("provider %q not found", name)
	}
	return p, nil
}

func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.providers))
	for k := range r.providers {
		out = append(out, k)
	}
	return out
}

func (r *Registry) Ready() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, p := range r.providers {
		if p.Ready() {
			return true
		}
	}
	return false
}
