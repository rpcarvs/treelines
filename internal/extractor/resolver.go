package extractor

import "github.com/rpcarvs/treelines/internal/model"

// Resolver performs name resolution for elements using both simple names
// and fully qualified names. When multiple elements share a name,
// the last one wins.
type Resolver struct {
	elements map[string]string
	fqNames  map[string]string
}

// NewResolver creates a Resolver populated with name-to-ID and fqname-to-ID mappings.
func NewResolver(elements []model.Element) *Resolver {
	m := make(map[string]string, len(elements))
	fq := make(map[string]string, len(elements))
	for _, e := range elements {
		m[e.Name] = e.ID
		if e.FQName != "" {
			fq[e.FQName] = e.ID
		}
	}
	return &Resolver{elements: m, fqNames: fq}
}

// Resolve returns the element ID for the given name, trying exact name
// match first, then falling back to FQName match.
func (r *Resolver) Resolve(name string) (string, bool) {
	if id, ok := r.elements[name]; ok {
		return id, ok
	}
	if id, ok := r.fqNames[name]; ok {
		return id, ok
	}
	return "", false
}

// ResolveQualified tries to resolve a qualified call like pkg.Function,
// module::function, or module.method using FQName lookups with
// language-appropriate separators.
func (r *Resolver) ResolveQualified(qualifier, name string) (string, bool) {
	candidates := []string{
		qualifier + "." + name,
		qualifier + "::" + name,
	}
	for _, fq := range candidates {
		if id, ok := r.fqNames[fq]; ok {
			return id, true
		}
	}
	return "", false
}
