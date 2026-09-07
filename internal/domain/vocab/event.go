package vocab

import "fmt"

// DomainEvent is one recorded domain event of a context: something
// that happened that the domain experts care about. RaisedBy names the
// aggregate whose operation raises it, when the model records that.
type DomainEvent struct {
	Name       string
	Definition string
	RaisedBy   string
	Line       int
}

// validate applies the domain event block's loader invariants.
func (e DomainEvent) validate(c BoundedContext) error {
	if err := requireDefinition(e.Line, "event", e.Name, e.Definition); err != nil {
		return err
	}
	if e.RaisedBy == "" {
		return nil
	}
	if _, ok := c.Aggregate(e.RaisedBy); !ok {
		return fmt.Errorf("domain_event/raised-by-an-aggregate: %scontext %q: event %q is raised by %q, which is not an aggregate of the context",
			at(e.Line), c.Name, e.Name, e.RaisedBy)
	}
	return nil
}
