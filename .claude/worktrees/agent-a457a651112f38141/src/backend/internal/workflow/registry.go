package workflow

import "fmt"

// Registry maps handler names to implementations. Populated at boot;
// read-only afterwards (no locking needed).
type Registry struct {
	handlers map[string]StepHandler
}

func NewRegistry() *Registry { return &Registry{handlers: map[string]StepHandler{}} }

func (r *Registry) Register(h StepHandler) {
	if _, dup := r.handlers[h.Name()]; dup {
		panic(fmt.Sprintf("workflow: duplicate handler %q", h.Name()))
	}
	r.handlers[h.Name()] = h
}

func (r *Registry) Get(name string) (StepHandler, bool) {
	h, ok := r.handlers[name]
	return h, ok
}

func (r *Registry) Names() []string {
	out := make([]string, 0, len(r.handlers))
	for n := range r.handlers {
		out = append(out, n)
	}
	return out
}
