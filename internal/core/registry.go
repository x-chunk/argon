package core

import (
	"fmt"
	"regexp"
	"sync"
)

// validName follows the MCP tool-name guidance: 1–128 chars of
// letters, digits, '_', '-' and '.'.
var validName = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,128}$`)

// Registry stores tools and resources in registration order. It is safe for
// concurrent use.
type Registry struct {
	mu    sync.RWMutex
	order []string
	tools map[string]*Tool

	resOrder  []string
	resources map[string]*Resource
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]*Tool), resources: make(map[string]*Resource)}
}

// AddResource registers r, keyed by its URI or URI template.
func (r *Registry) AddResource(res *Resource) error {
	if err := res.validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, dup := r.resources[res.Key()]; dup {
		return fmt.Errorf("argon: duplicate resource %q", res.Key())
	}
	r.resources[res.Key()] = res
	r.resOrder = append(r.resOrder, res.Key())
	return nil
}

// Resources returns resources and templates in registration order.
func (r *Registry) Resources() []*Resource {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Resource, 0, len(r.resOrder))
	for _, k := range r.resOrder {
		out = append(out, r.resources[k])
	}
	return out
}

// Add registers t. It fails on invalid or duplicate names and missing handlers.
func (r *Registry) Add(t *Tool) error {
	if !validName.MatchString(t.Name) {
		return fmt.Errorf("argon: invalid tool name %q", t.Name)
	}
	if t.Handler == nil {
		return fmt.Errorf("argon: tool %q has no handler", t.Name)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, dup := r.tools[t.Name]; dup {
		return fmt.Errorf("argon: duplicate tool %q", t.Name)
	}
	r.tools[t.Name] = t
	r.order = append(r.order, t.Name)
	return nil
}

// Get returns the tool named name.
func (r *Registry) Get(name string) (*Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

// All returns the tools in registration order.
func (r *Registry) All() []*Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Tool, 0, len(r.order))
	for _, n := range r.order {
		out = append(out, r.tools[n])
	}
	return out
}
