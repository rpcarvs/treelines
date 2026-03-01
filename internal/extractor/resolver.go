package extractor

import "lines/internal/model"

// Resolver performs simple exact-match name resolution for elements.
// When multiple elements share a name, the last one wins.
type Resolver struct {
	elements map[string]string
}

// NewResolver creates a Resolver populated with name-to-ID mappings.
func NewResolver(elements []model.Element) *Resolver {
	m := make(map[string]string, len(elements))
	for _, e := range elements {
		m[e.Name] = e.ID
	}
	return &Resolver{elements: m}
}

// Resolve returns the element ID for the given name, if found.
func (r *Resolver) Resolve(name string) (string, bool) {
	id, ok := r.elements[name]
	return id, ok
}
