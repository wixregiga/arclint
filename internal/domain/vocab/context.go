package vocab

// BoundedContext is one explicit model boundary: a name plus the four
// file-recordable sections (entities, value_objects, invariants, events).
// Line is where the context is written down in the recorded Ubiquitous
// Language file, 0 for a context that is not written down yet.
type BoundedContext struct {
	Name         string
	Entities     []Entity
	ValueObjects []Definition
	Invariants   []Invariant
	Events       []Definition
	Line         int
}

func cloneContext(c BoundedContext) BoundedContext {
	return BoundedContext{
		Name:         c.Name,
		Entities:     cloneEntities(c.Entities),
		ValueObjects: cloneDefs(c.ValueObjects),
		Invariants:   cloneInvariants(c.Invariants),
		Events:       cloneDefs(c.Events),
		Line:         c.Line,
	}
}

func cloneContexts(in []BoundedContext) []BoundedContext {
	if in == nil {
		return nil
	}
	out := make([]BoundedContext, len(in))
	for i, c := range in {
		out[i] = cloneContext(c)
	}
	return out
}

func cloneInvariants(in []Invariant) []Invariant {
	if in == nil {
		return nil
	}
	out := make([]Invariant, len(in))
	copy(out, in)
	return out
}

func contextIsEmpty(c BoundedContext) bool {
	return len(c.Entities) == 0 &&
		len(c.ValueObjects) == 0 &&
		len(c.Invariants) == 0 &&
		len(c.Events) == 0
}
