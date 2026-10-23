package argon

import (
	"fmt"
	"regexp"
	"sync"
)

// validName follows the MCP tool-name guidance: 1–128 chars of
// letters, digits, '_', '-' and '.'.
var validName = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,128}$`)

// registry stores tools, resources and prompts in registration order. Its
// lock lets built servers read it while it is not being written; Server
// itself does not support concurrent registration.
type registry struct {
	mu    sync.RWMutex
	order []string
	tools map[string]*toolDef

	resOrder  []string
	resources map[string]*resourceDef

	promptOrder []string
	prompts     map[string]*promptDef
}

// newRegistry returns an empty registry.
func newRegistry() *registry {
	return &registry{
		tools:     make(map[string]*toolDef),
		resources: make(map[string]*resourceDef),
		prompts:   make(map[string]*promptDef),
	}
}

// AddPrompt registers p, keyed by its name.
func (r *registry) AddPrompt(p *promptDef) error {
	if err := p.validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, dup := r.prompts[p.Name]; dup {
		return fmt.Errorf("argon: duplicate prompt %q", p.Name)
	}
	r.prompts[p.Name] = p
	r.promptOrder = append(r.promptOrder, p.Name)
	return nil
}

// Prompts returns prompts in registration order.
func (r *registry) Prompts() []*promptDef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*promptDef, 0, len(r.promptOrder))
	for _, n := range r.promptOrder {
		out = append(out, r.prompts[n])
	}
	return out
}

// AddResource registers r, keyed by its URI or URI template.
func (r *registry) AddResource(res *resourceDef) error {
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
func (r *registry) Resources() []*resourceDef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*resourceDef, 0, len(r.resOrder))
	for _, k := range r.resOrder {
		out = append(out, r.resources[k])
	}
	return out
}

// Add registers t. It fails on invalid or duplicate names and missing handlers.
func (r *registry) Add(t *toolDef) error {
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
func (r *registry) Get(name string) (*toolDef, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

// All returns the tools in registration order.
func (r *registry) All() []*toolDef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*toolDef, 0, len(r.order))
	for _, n := range r.order {
		out = append(out, r.tools[n])
	}
	return out
}
