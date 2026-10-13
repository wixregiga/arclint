package cmlvocab

// Property test of Record over long chains of edits. Each chain starts from a
// copy of a committed recording (arclint's own and the boxoffice proving
// ground) and applies random language edits one at a time: add, change, remove,
// rename and reorder across every concept the vocabulary records, relations and
// the project included. After every step the file must read back as exactly the
// requested language, parse and pass the Context Mapper checks, be a formatter
// fixed point, differ from the previous step only in the elements that differ
// between the two languages, stay untouched by a second Record of the same
// language, and return to the previous bytes when the previous language is
// recorded again.

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/wixregiga/arclint/internal/domain/vocab"
	"github.com/wixregiga/arclint/internal/infrastructure/vocab/cml/syntax"
)

// editChainSteps is the length of each chain in an exhaustive run; it must
// stay at 300 or more edits per recording. Other runs, the repository gate
// and CI included, check a fiftieth of that.
const editChainSteps = 300

var errNoCandidate = errors.New("no element to edit")

type editChain struct {
	rng     *rand.Rand
	counter int
}

func (c *editChain) fresh(prefix string) string {
	c.counter++
	return fmt.Sprintf("%s%d", prefix, c.counter)
}

var editPhrases = []string{
	"A plain fact of the domain.",
	"Quotes \" and a backslash \\ inside.",
	"First line.\nSecond line.\tTabbed.",
	"Unicode 日本語 and café.",
	"Percent %, comma , and equals = signs.",
	"A longer definition that says what the term means to the people who use it and why the model keeps it apart from its neighbours.",
}

func (c *editChain) text() string {
	c.counter++
	return fmt.Sprintf("%s (%d)", editPhrases[c.rng.IntN(len(editPhrases))], c.counter)
}

func (c *editChain) aliases() []string {
	out := make([]string, c.rng.IntN(3))
	for i := range out {
		out[i] = c.fresh("alias, = % ")
	}
	return out
}

// site locates one recorded entry by its context, owner and name.
type site struct{ context, owner, name string }

func (s site) locator() vocab.Locator {
	return vocab.Locator{Context: s.context, Owner: s.owner, Name: s.name}
}

type inventory struct {
	contexts, aggregates, entities, values, specs, events, services, questions []site
	aggInvariants, valueInvariants, aggAssertions, svcAssertions               []site
}

func inventoryOf(l vocab.UbiquitousLanguage) inventory {
	var inv inventory
	for _, c := range l.Contexts {
		inv.contexts = append(inv.contexts, site{context: c.Name, name: c.Name})
		for _, a := range c.Aggregates {
			inv.aggregates = append(inv.aggregates, site{context: c.Name, name: a.Name})
			for _, e := range a.Entities {
				inv.entities = append(inv.entities, site{c.Name, a.Name, e.Name})
			}
			for _, i := range a.Invariants {
				inv.aggInvariants = append(inv.aggInvariants, site{c.Name, a.Name, i.Key})
			}
			for _, as := range a.Assertions {
				inv.aggAssertions = append(inv.aggAssertions, site{c.Name, a.Name, as.Key})
			}
		}
		for _, v := range c.ValueObjects {
			inv.values = append(inv.values, site{context: c.Name, name: v.Name})
			for _, i := range v.Invariants {
				inv.valueInvariants = append(inv.valueInvariants, site{c.Name, v.Name, i.Key})
			}
		}
		for _, s := range c.Specifications {
			inv.specs = append(inv.specs, site{context: c.Name, name: s.Name})
		}
		for _, e := range c.Events {
			inv.events = append(inv.events, site{context: c.Name, name: e.Name})
		}
		for _, s := range c.Services {
			inv.services = append(inv.services, site{context: c.Name, name: s.Name})
			for _, as := range s.Assertions {
				inv.svcAssertions = append(inv.svcAssertions, site{c.Name, s.Name, as.Key})
			}
		}
		for _, q := range c.Questions {
			inv.questions = append(inv.questions, site{context: c.Name, name: q.Key})
		}
	}
	return inv
}

func (c *editChain) one(sites []site) (site, error) {
	if len(sites) == 0 {
		return site{}, errNoCandidate
	}
	return sites[c.rng.IntN(len(sites))], nil
}

func (c *editChain) maybe(n int) bool { return c.rng.IntN(n) == 0 }

func define(l vocab.UbiquitousLanguage, k vocab.Concept, at vocab.Locator, ch vocab.Change) (vocab.UbiquitousLanguage, error) {
	next, _, err := l.Define(k, at, ch)
	return next, err
}

func remove(l vocab.UbiquitousLanguage, k vocab.Concept, at vocab.Locator) (vocab.UbiquitousLanguage, error) {
	next, _, err := l.Remove(k, at)
	return next, err
}

// mutate changes a validated copy of the language directly, for the edits the
// vocabulary's Define and Remove do not offer (relations, the project, order,
// names), and validates the result.
func mutate(l vocab.UbiquitousLanguage, change func(*vocab.UbiquitousLanguage) error) (vocab.UbiquitousLanguage, error) {
	clone, err := vocab.NewUbiquitousLanguage(l.Project, l.Description, l.Contexts, l.Relations)
	if err != nil {
		return vocab.UbiquitousLanguage{}, err
	}
	if err := change(&clone); err != nil {
		return vocab.UbiquitousLanguage{}, err
	}
	return vocab.NewUbiquitousLanguage(clone.Project, clone.Description, clone.Contexts, clone.Relations)
}

func contextIndex(l *vocab.UbiquitousLanguage, name string) int {
	for i := range l.Contexts {
		if l.Contexts[i].Name == name {
			return i
		}
	}
	return -1
}

var operationNames = []string{"Publish", "Place", "Cancel", "Ship", "Pay", "Close"}

func (c *editChain) operation() string {
	if c.maybe(4) {
		return c.fresh("ZOp")
	}
	return operationNames[c.rng.IntN(len(operationNames))]
}

func (c *editChain) repository(name string) string {
	switch c.rng.IntN(3) {
	case 0:
		return ""
	case 1:
		return name + "Repository"
	default:
		return c.fresh("storage name ")
	}
}

// identityFor names an aggregate's or member's identity: a recorded value
// object of the context, or a fresh one the vocabulary implies.
func (c *editChain) identityFor(l vocab.UbiquitousLanguage, context string) string {
	if i := contextIndex(&l, context); i >= 0 && len(l.Contexts[i].ValueObjects) > 0 && c.maybe(2) {
		return l.Contexts[i].ValueObjects[c.rng.IntN(len(l.Contexts[i].ValueObjects))].Name
	}
	return c.fresh("ZIdentity")
}

type namedEdit struct {
	name string
	run  func(c *editChain, l vocab.UbiquitousLanguage) (vocab.UbiquitousLanguage, error)
}

func defining(text string) vocab.Change {
	return vocab.Change{SetDefinition: true, Definition: text}
}

func editCatalog() []namedEdit {
	return []namedEdit{
		{"add context", func(c *editChain, l vocab.UbiquitousLanguage) (vocab.UbiquitousLanguage, error) {
			return define(l, vocab.ConceptBoundedContext, vocab.Locator{Name: c.fresh("zc")}, defining(c.text()))
		}},
		{"change context", func(c *editChain, l vocab.UbiquitousLanguage) (vocab.UbiquitousLanguage, error) {
			s, err := c.one(inventoryOf(l).contexts)
			if err != nil {
				return l, err
			}
			return define(l, vocab.ConceptBoundedContext, s.locator(), defining(c.text()))
		}},
		{"remove context", func(c *editChain, l vocab.UbiquitousLanguage) (vocab.UbiquitousLanguage, error) {
			if len(l.Contexts) < 3 {
				return l, errNoCandidate
			}
			s, err := c.one(inventoryOf(l).contexts)
			if err != nil {
				return l, err
			}
			return remove(l, vocab.ConceptBoundedContext, s.locator())
		}},
		{"add aggregate", func(c *editChain, l vocab.UbiquitousLanguage) (vocab.UbiquitousLanguage, error) {
			s, err := c.one(inventoryOf(l).contexts)
			if err != nil {
				return l, err
			}
			name := c.fresh("ZAggregate")
			ch := vocab.Change{SetDefinition: true, Definition: c.text(), SetIdentity: true, Identity: c.identityFor(l, s.context)}
			if c.maybe(2) {
				ch.SetAliases, ch.Aliases = true, c.aliases()
			}
			if c.maybe(2) {
				ch.SetRepository, ch.Repository = true, c.repository(name)
			}
			if c.maybe(3) {
				ch.SetFactory, ch.Factory = true, c.fresh("ZFactory")
			}
			return define(l, vocab.ConceptAggregate, vocab.Locator{Context: s.context, Name: name}, ch)
		}},
		{"change aggregate", func(c *editChain, l vocab.UbiquitousLanguage) (vocab.UbiquitousLanguage, error) {
			s, err := c.one(inventoryOf(l).aggregates)
			if err != nil {
				return l, err
			}
			var ch vocab.Change
			switch c.rng.IntN(5) {
			case 0:
				ch = defining(c.text())
			case 1:
				ch = vocab.Change{SetIdentity: true, Identity: c.identityFor(l, s.context)}
			case 2:
				ch = vocab.Change{SetAliases: true, Aliases: c.aliases()}
			case 3:
				ch = vocab.Change{SetRepository: true, Repository: c.repository(s.name)}
			default:
				ch = vocab.Change{SetFactory: true}
				if c.maybe(2) {
					ch.Factory = c.fresh("ZFactory")
				}
			}
			return define(l, vocab.ConceptAggregate, s.locator(), ch)
		}},
		{"remove aggregate", func(c *editChain, l vocab.UbiquitousLanguage) (vocab.UbiquitousLanguage, error) {
			s, err := c.one(inventoryOf(l).aggregates)
			if err != nil {
				return l, err
			}
			return remove(l, vocab.ConceptAggregate, s.locator())
		}},
		{"add entity", func(c *editChain, l vocab.UbiquitousLanguage) (vocab.UbiquitousLanguage, error) {
			s, err := c.one(inventoryOf(l).aggregates)
			if err != nil {
				return l, err
			}
			ch := defining(c.text())
			if c.maybe(2) {
				ch.SetIdentity, ch.Identity = true, c.identityFor(l, s.context)
			}
			if c.maybe(2) {
				ch.SetAliases, ch.Aliases = true, c.aliases()
			}
			return define(l, vocab.ConceptEntity, vocab.Locator{Context: s.context, Owner: s.name, Name: c.fresh("ZEntity")}, ch)
		}},
		{"change entity", func(c *editChain, l vocab.UbiquitousLanguage) (vocab.UbiquitousLanguage, error) {
			s, err := c.one(inventoryOf(l).entities)
			if err != nil {
				return l, err
			}
			var ch vocab.Change
			switch c.rng.IntN(3) {
			case 0:
				ch = defining(c.text())
			case 1:
				ch = vocab.Change{SetIdentity: true, Identity: c.identityFor(l, s.context)}
			default:
				ch = vocab.Change{SetAliases: true, Aliases: c.aliases()}
			}
			return define(l, vocab.ConceptEntity, s.locator(), ch)
		}},
		{"remove entity", func(c *editChain, l vocab.UbiquitousLanguage) (vocab.UbiquitousLanguage, error) {
			s, err := c.one(inventoryOf(l).entities)
			if err != nil {
				return l, err
			}
			return remove(l, vocab.ConceptEntity, s.locator())
		}},
		{"add value object", func(c *editChain, l vocab.UbiquitousLanguage) (vocab.UbiquitousLanguage, error) {
			s, err := c.one(inventoryOf(l).contexts)
			if err != nil {
				return l, err
			}
			ch := defining(c.text())
			if c.maybe(2) {
				ch.SetAliases, ch.Aliases = true, c.aliases()
			}
			return define(l, vocab.ConceptValueObject, vocab.Locator{Context: s.context, Name: c.fresh("ZValue")}, ch)
		}},
		{"change value object", func(c *editChain, l vocab.UbiquitousLanguage) (vocab.UbiquitousLanguage, error) {
			s, err := c.one(inventoryOf(l).values)
			if err != nil {
				return l, err
			}
			ch := defining(c.text())
			if c.maybe(2) {
				ch = vocab.Change{SetAliases: true, Aliases: c.aliases()}
			}
			return define(l, vocab.ConceptValueObject, s.locator(), ch)
		}},
		{"remove value object", func(c *editChain, l vocab.UbiquitousLanguage) (vocab.UbiquitousLanguage, error) {
			s, err := c.one(inventoryOf(l).values)
			if err != nil {
				return l, err
			}
			return remove(l, vocab.ConceptValueObject, s.locator())
		}},
		{"add invariant", func(c *editChain, l vocab.UbiquitousLanguage) (vocab.UbiquitousLanguage, error) {
			inv := inventoryOf(l)
			owners := append(append([]site(nil), inv.aggregates...), inv.values...)
			s, err := c.one(owners)
			if err != nil {
				return l, err
			}
			at := vocab.Locator{Context: s.context, Owner: s.name, Name: c.fresh("zinv-")}
			return define(l, vocab.ConceptInvariant, at, vocab.Change{SetStatement: true, Statement: c.text()})
		}},
		{"change invariant", func(c *editChain, l vocab.UbiquitousLanguage) (vocab.UbiquitousLanguage, error) {
			inv := inventoryOf(l)
			s, err := c.one(append(append([]site(nil), inv.aggInvariants...), inv.valueInvariants...))
			if err != nil {
				return l, err
			}
			return define(l, vocab.ConceptInvariant, s.locator(), vocab.Change{SetStatement: true, Statement: c.text()})
		}},
		{"remove invariant", func(c *editChain, l vocab.UbiquitousLanguage) (vocab.UbiquitousLanguage, error) {
			inv := inventoryOf(l)
			s, err := c.one(append(append([]site(nil), inv.aggInvariants...), inv.valueInvariants...))
			if err != nil {
				return l, err
			}
			return remove(l, vocab.ConceptInvariant, s.locator())
		}},
		{"add assertion", func(c *editChain, l vocab.UbiquitousLanguage) (vocab.UbiquitousLanguage, error) {
			inv := inventoryOf(l)
			s, err := c.one(append(append([]site(nil), inv.aggregates...), inv.services...))
			if err != nil {
				return l, err
			}
			at := vocab.Locator{Context: s.context, Owner: s.name, Name: c.fresh("zasr-")}
			return define(l, vocab.ConceptAssertion, at, vocab.Change{SetOn: true, On: c.operation(), SetStatement: true, Statement: c.text()})
		}},
		{"change assertion", func(c *editChain, l vocab.UbiquitousLanguage) (vocab.UbiquitousLanguage, error) {
			inv := inventoryOf(l)
			s, err := c.one(append(append([]site(nil), inv.aggAssertions...), inv.svcAssertions...))
			if err != nil {
				return l, err
			}
			ch := vocab.Change{SetStatement: true, Statement: c.text()}
			if c.maybe(2) {
				ch = vocab.Change{SetOn: true, On: c.operation()}
			}
			return define(l, vocab.ConceptAssertion, s.locator(), ch)
		}},
		{"remove assertion", func(c *editChain, l vocab.UbiquitousLanguage) (vocab.UbiquitousLanguage, error) {
			inv := inventoryOf(l)
			s, err := c.one(append(append([]site(nil), inv.aggAssertions...), inv.svcAssertions...))
			if err != nil {
				return l, err
			}
			return remove(l, vocab.ConceptAssertion, s.locator())
		}},
		{"add event", func(c *editChain, l vocab.UbiquitousLanguage) (vocab.UbiquitousLanguage, error) {
			s, err := c.one(inventoryOf(l).contexts)
			if err != nil {
				return l, err
			}
			ch := defining(c.text())
			if i := contextIndex(&l, s.context); len(l.Contexts[i].Aggregates) > 0 && c.maybe(2) {
				ch.SetRaisedBy, ch.RaisedBy = true, l.Contexts[i].Aggregates[c.rng.IntN(len(l.Contexts[i].Aggregates))].Name
			}
			return define(l, vocab.ConceptDomainEvent, vocab.Locator{Context: s.context, Name: c.fresh("ZEvent")}, ch)
		}},
		{"change event", func(c *editChain, l vocab.UbiquitousLanguage) (vocab.UbiquitousLanguage, error) {
			s, err := c.one(inventoryOf(l).events)
			if err != nil {
				return l, err
			}
			ch := defining(c.text())
			if i := contextIndex(&l, s.context); c.maybe(2) {
				ch = vocab.Change{SetRaisedBy: true}
				if aggregates := l.Contexts[i].Aggregates; len(aggregates) > 0 && !c.maybe(4) {
					ch.RaisedBy = aggregates[c.rng.IntN(len(aggregates))].Name
				}
			}
			return define(l, vocab.ConceptDomainEvent, s.locator(), ch)
		}},
		{"remove event", func(c *editChain, l vocab.UbiquitousLanguage) (vocab.UbiquitousLanguage, error) {
			s, err := c.one(inventoryOf(l).events)
			if err != nil {
				return l, err
			}
			return remove(l, vocab.ConceptDomainEvent, s.locator())
		}},
		{"add service", func(c *editChain, l vocab.UbiquitousLanguage) (vocab.UbiquitousLanguage, error) {
			s, err := c.one(inventoryOf(l).contexts)
			if err != nil {
				return l, err
			}
			return define(l, vocab.ConceptDomainService, vocab.Locator{Context: s.context, Name: c.fresh("ZService")}, defining(c.text()))
		}},
		{"change service", func(c *editChain, l vocab.UbiquitousLanguage) (vocab.UbiquitousLanguage, error) {
			s, err := c.one(inventoryOf(l).services)
			if err != nil {
				return l, err
			}
			return define(l, vocab.ConceptDomainService, s.locator(), defining(c.text()))
		}},
		{"remove service", func(c *editChain, l vocab.UbiquitousLanguage) (vocab.UbiquitousLanguage, error) {
			s, err := c.one(inventoryOf(l).services)
			if err != nil {
				return l, err
			}
			return remove(l, vocab.ConceptDomainService, s.locator())
		}},
		{"add specification", func(c *editChain, l vocab.UbiquitousLanguage) (vocab.UbiquitousLanguage, error) {
			s, err := c.one(inventoryOf(l).contexts)
			if err != nil {
				return l, err
			}
			return define(l, vocab.ConceptSpecification, vocab.Locator{Context: s.context, Name: c.fresh("ZSpecification")}, defining(c.text()))
		}},
		{"change specification", func(c *editChain, l vocab.UbiquitousLanguage) (vocab.UbiquitousLanguage, error) {
			s, err := c.one(inventoryOf(l).specs)
			if err != nil {
				return l, err
			}
			return define(l, vocab.ConceptSpecification, s.locator(), defining(c.text()))
		}},
		{"remove specification", func(c *editChain, l vocab.UbiquitousLanguage) (vocab.UbiquitousLanguage, error) {
			s, err := c.one(inventoryOf(l).specs)
			if err != nil {
				return l, err
			}
			return remove(l, vocab.ConceptSpecification, s.locator())
		}},
		{"add question", func(c *editChain, l vocab.UbiquitousLanguage) (vocab.UbiquitousLanguage, error) {
			s, err := c.one(inventoryOf(l).contexts)
			if err != nil {
				return l, err
			}
			return define(l, vocab.ConceptQuestion, vocab.Locator{Context: s.context, Name: c.fresh("zq-")}, vocab.Change{SetText: true, Text: c.text()})
		}},
		{"change question", func(c *editChain, l vocab.UbiquitousLanguage) (vocab.UbiquitousLanguage, error) {
			s, err := c.one(inventoryOf(l).questions)
			if err != nil {
				return l, err
			}
			return define(l, vocab.ConceptQuestion, s.locator(), vocab.Change{SetText: true, Text: c.text()})
		}},
		{"remove question", func(c *editChain, l vocab.UbiquitousLanguage) (vocab.UbiquitousLanguage, error) {
			s, err := c.one(inventoryOf(l).questions)
			if err != nil {
				return l, err
			}
			return remove(l, vocab.ConceptQuestion, s.locator())
		}},
		{"add relation", func(c *editChain, l vocab.UbiquitousLanguage) (vocab.UbiquitousLanguage, error) {
			return mutate(l, func(m *vocab.UbiquitousLanguage) error {
				if len(m.Contexts) < 2 {
					return errNoCandidate
				}
				from, to := m.Contexts[c.rng.IntN(len(m.Contexts))].Name, m.Contexts[c.rng.IntN(len(m.Contexts))].Name
				kinds := vocab.RelationKinds()
				r := vocab.ContextRelation{From: from, To: to, Kind: kinds[c.rng.IntN(len(kinds))]}
				if c.maybe(2) {
					r.Description = c.text()
				}
				m.Relations = append(m.Relations, r)
				return nil
			})
		}},
		{"change relation", func(c *editChain, l vocab.UbiquitousLanguage) (vocab.UbiquitousLanguage, error) {
			return mutate(l, func(m *vocab.UbiquitousLanguage) error {
				if len(m.Relations) == 0 {
					return errNoCandidate
				}
				r := &m.Relations[c.rng.IntN(len(m.Relations))]
				switch c.rng.IntN(3) {
				case 0:
					kinds := vocab.RelationKinds()
					r.Kind = kinds[c.rng.IntN(len(kinds))]
				case 1:
					r.Description = c.text()
				default:
					r.From, r.To = r.To, r.From
				}
				return nil
			})
		}},
		{"remove relation", func(c *editChain, l vocab.UbiquitousLanguage) (vocab.UbiquitousLanguage, error) {
			return mutate(l, func(m *vocab.UbiquitousLanguage) error {
				if len(m.Relations) == 0 {
					return errNoCandidate
				}
				i := c.rng.IntN(len(m.Relations))
				m.Relations = append(m.Relations[:i], m.Relations[i+1:]...)
				return nil
			})
		}},
		{"change project", func(c *editChain, l vocab.UbiquitousLanguage) (vocab.UbiquitousLanguage, error) {
			return mutate(l, func(m *vocab.UbiquitousLanguage) error {
				if c.maybe(3) {
					m.Project = c.fresh("zproject")
				} else {
					m.Description = c.text()
				}
				return nil
			})
		}},
		{"reorder a section", reorderSection},
		{"rename an element", renameElement},
	}
}

// sectionsOf lists every ordered section of the language by its length and the
// swap that exchanges two of its entries.
func sectionsOf(m *vocab.UbiquitousLanguage) []struct {
	length int
	swap   func(i, j int)
} {
	type section = struct {
		length int
		swap   func(i, j int)
	}
	var out []section
	add := func(length int, swap func(i, j int)) {
		if length > 1 {
			out = append(out, section{length, swap})
		}
	}
	add(len(m.Contexts), func(i, j int) { m.Contexts[i], m.Contexts[j] = m.Contexts[j], m.Contexts[i] })
	add(len(m.Relations), func(i, j int) { m.Relations[i], m.Relations[j] = m.Relations[j], m.Relations[i] })
	for ci := range m.Contexts {
		c := &m.Contexts[ci]
		add(len(c.Aggregates), func(i, j int) { c.Aggregates[i], c.Aggregates[j] = c.Aggregates[j], c.Aggregates[i] })
		add(len(c.ValueObjects), func(i, j int) { c.ValueObjects[i], c.ValueObjects[j] = c.ValueObjects[j], c.ValueObjects[i] })
		add(len(c.Specifications), func(i, j int) { c.Specifications[i], c.Specifications[j] = c.Specifications[j], c.Specifications[i] })
		add(len(c.Events), func(i, j int) { c.Events[i], c.Events[j] = c.Events[j], c.Events[i] })
		add(len(c.Services), func(i, j int) { c.Services[i], c.Services[j] = c.Services[j], c.Services[i] })
		add(len(c.Questions), func(i, j int) { c.Questions[i], c.Questions[j] = c.Questions[j], c.Questions[i] })
		for ai := range c.Aggregates {
			a := &c.Aggregates[ai]
			add(len(a.Entities), func(i, j int) { a.Entities[i], a.Entities[j] = a.Entities[j], a.Entities[i] })
			add(len(a.Invariants), func(i, j int) { a.Invariants[i], a.Invariants[j] = a.Invariants[j], a.Invariants[i] })
			add(len(a.Assertions), func(i, j int) { a.Assertions[i], a.Assertions[j] = a.Assertions[j], a.Assertions[i] })
		}
		for vi := range c.ValueObjects {
			v := &c.ValueObjects[vi]
			add(len(v.Invariants), func(i, j int) { v.Invariants[i], v.Invariants[j] = v.Invariants[j], v.Invariants[i] })
		}
		for si := range c.Services {
			s := &c.Services[si]
			add(len(s.Assertions), func(i, j int) { s.Assertions[i], s.Assertions[j] = s.Assertions[j], s.Assertions[i] })
		}
	}
	return out
}

func reorderSection(c *editChain, l vocab.UbiquitousLanguage) (vocab.UbiquitousLanguage, error) {
	return mutate(l, func(m *vocab.UbiquitousLanguage) error {
		sections := sectionsOf(m)
		if len(sections) == 0 {
			return errNoCandidate
		}
		s := sections[c.rng.IntN(len(sections))]
		c.rng.Shuffle(s.length, s.swap)
		return nil
	})
}

// renameElement renames one element and repairs the references to it that the
// language checks, so the result is the same element under a new name.
func renameElement(c *editChain, l vocab.UbiquitousLanguage) (vocab.UbiquitousLanguage, error) {
	return mutate(l, func(m *vocab.UbiquitousLanguage) error {
		inv := inventoryOf(*m)
		kinds := []struct {
			sites []site
			apply func(s site, name string)
			base  string
		}{
			{inv.contexts, func(s site, name string) {
				m.Contexts[contextIndex(m, s.context)].Name = name
				for i := range m.Relations {
					if m.Relations[i].From == s.name {
						m.Relations[i].From = name
					}
					if m.Relations[i].To == s.name {
						m.Relations[i].To = name
					}
				}
			}, "zc"},
			{inv.aggregates, func(s site, name string) {
				ctx := &m.Contexts[contextIndex(m, s.context)]
				for i := range ctx.Aggregates {
					if ctx.Aggregates[i].Name == s.name {
						ctx.Aggregates[i].Name = name
					}
				}
				for i := range ctx.Events {
					if ctx.Events[i].RaisedBy == s.name {
						ctx.Events[i].RaisedBy = name
					}
				}
			}, "ZAggregate"},
			{inv.entities, func(s site, name string) {
				ctx := &m.Contexts[contextIndex(m, s.context)]
				for ai := range ctx.Aggregates {
					for ei := range ctx.Aggregates[ai].Entities {
						if ctx.Aggregates[ai].Entities[ei].Name == s.name {
							ctx.Aggregates[ai].Entities[ei].Name = name
						}
					}
				}
			}, "ZEntity"},
			{inv.values, func(s site, name string) {
				ctx := &m.Contexts[contextIndex(m, s.context)]
				for i := range ctx.ValueObjects {
					if ctx.ValueObjects[i].Name == s.name {
						ctx.ValueObjects[i].Name = name
					}
				}
			}, "ZValue"},
			{inv.specs, func(s site, name string) {
				ctx := &m.Contexts[contextIndex(m, s.context)]
				for i := range ctx.Specifications {
					if ctx.Specifications[i].Name == s.name {
						ctx.Specifications[i].Name = name
					}
				}
			}, "ZSpecification"},
			{inv.events, func(s site, name string) {
				ctx := &m.Contexts[contextIndex(m, s.context)]
				for i := range ctx.Events {
					if ctx.Events[i].Name == s.name {
						ctx.Events[i].Name = name
					}
				}
			}, "ZEvent"},
			{inv.services, func(s site, name string) {
				ctx := &m.Contexts[contextIndex(m, s.context)]
				for i := range ctx.Services {
					if ctx.Services[i].Name == s.name {
						ctx.Services[i].Name = name
					}
				}
			}, "ZService"},
		}
		k := kinds[c.rng.IntN(len(kinds))]
		s, err := c.one(k.sites)
		if err != nil {
			return err
		}
		k.apply(s, c.fresh(k.base))
		return nil
	})
}

// A reader of the chain's file: bytes, modification time and identity.
type fileState struct {
	data []byte
	info os.FileInfo
}

func stateOf(t *testing.T, path string) fileState {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return fileState{data, info}
}

// differsFrom says how the file at path is no longer byte-for-byte, mtime and
// inode what it was; "" when it is untouched.
func (before fileState) differsFrom(path string) string {
	after, err := os.Stat(path)
	if err != nil {
		return err.Error()
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err.Error()
	}
	switch {
	case !bytes.Equal(before.data, data):
		return "the bytes changed"
	case !before.info.ModTime().Equal(after.ModTime()):
		return fmt.Sprintf("the modification time moved from %v to %v", before.info.ModTime(), after.ModTime())
	case !os.SameFile(before.info, after):
		return "the file was replaced"
	}
	return ""
}

// longAgo is the modification time the tests give a recording before they
// record again: a write of any kind moves it.
var longAgo = time.Date(2001, time.February, 3, 4, 5, 6, 0, time.UTC)

func backdate(t *testing.T, path string) {
	t.Helper()
	if err := os.Chtimes(path, longAgo, longAgo); err != nil {
		t.Fatal(err)
	}
}

// removesElements reports whether the edit from before to after leaves out an
// element that stood in before.
func removesElements(before, after recordingLayout) bool {
	kept := modelOf(after.lang).values
	for key := range modelOf(before.lang).values {
		if _, ok := kept[key]; !ok {
			return true
		}
	}
	return false
}

// residueViolations lists the lines in which restored differs from before,
// except for the operation declarations that the forward edit introduced for
// the On name of an assertion, and the braces of the value object or service
// that holds them. Record keeps such an operation when the assertion that
// named it goes: the declaration may be authored CML, and a removal that
// deleted it would delete foreign content.
func residueViolations(before, restored recordingLayout, forward editScope) (violations []string, residue bool) {
	existing := map[string]bool{}
	for _, owner := range before.owners {
		existing[owner] = true
	}
	excused := map[string]bool{}
	for key := range forward.operations {
		if !existing["operation:"+key] {
			excused[key] = true
		}
	}
	allows := func(owner string) bool {
		if rest, ok := strings.CutPrefix(owner, "operation:"); ok {
			return excused[rest]
		}
		if inner, ok := strings.CutPrefix(owner, keyFrame); ok {
			if kind, rest, _ := strings.Cut(inner, ":"); kind == "service" || kind == "value" {
				for key := range excused {
					if strings.HasPrefix(key, rest+"/") {
						return true
					}
				}
			}
		}
		return false
	}
	deleted, inserted := lcsDiff(before.keyed(), restored.keyed())
	for _, line := range deleted {
		if !allows(line.key) {
			violations = append(violations, fmt.Sprintf("-%d [%s] %s", line.number, line.key, strings.TrimSpace(line.text)))
		}
	}
	for _, line := range inserted {
		if !allows(line.key) {
			violations = append(violations, fmt.Sprintf("+%d [%s] %s", line.number, line.key, strings.TrimSpace(line.text)))
		}
	}
	return violations, len(deleted)+len(inserted) > 0
}

func TestEditSequencesStayLocalAndReversible(t *testing.T) {
	steps := editChainSteps
	reduced := !exhaustive()
	if reduced {
		steps /= 50
	}
	sources := []struct{ name, path string }{
		{"arclint", "../../../../domain.arclint.cml"},
		{"boxoffice", "../../../../testing/boxoffice/domain.arclint.cml"},
	}
	for index, source := range sources {
		// A reduced run walks the smaller recording only; each chain keeps
		// its own seed either way.
		if reduced && source.name != "boxoffice" {
			continue
		}
		t.Run(source.name, func(t *testing.T) {
			t.Parallel()
			committed, err := os.ReadFile(source.path)
			if err != nil {
				t.Fatal(err)
			}
			repo := syncFixtureRepository(t, string(committed))
			chain := &editChain{rng: rand.New(rand.NewPCG(20261012, uint64(index+1)))}
			catalog := editCatalog()
			layout := layoutOf(t, committed)
			language := layout.lang
			// The chain adds and removes in turn; keeping its size between these bounds
			// keeps the committed content in play instead of letting the walk empty the
			// file or grow it without end.
			floor, ceiling := languageSize(language)/2, languageSize(language)+30
			applied := map[string]int{}
			skipped := 0
			step := 0
			for attempt := 0; step < steps; attempt++ {
				if attempt > 40*steps {
					t.Fatalf("only %d of %d edits applied after %d attempts", step, steps, attempt)
				}
				edit := catalog[chain.rng.IntN(len(catalog))]
				if size := languageSize(language); strings.HasPrefix(edit.name, "add ") && size >= ceiling || strings.HasPrefix(edit.name, "remove ") && size <= floor {
					skipped++
					continue
				}
				next, err := edit.run(chain, language)
				if err != nil || language.Difference(next) == "" {
					skipped++
					continue
				}
				step++
				var failures []string
				failures, layout = editStepFailures(t, repo, layout, language, next)
				if len(failures) > 0 {
					t.Fatalf("step %d (%s):\n%s", step, edit.name, strings.Join(failures, "\n"))
				}
				applied[edit.name]++
				language = next
				if step%20 == 0 || step == steps {
					exportOracleCopy(t, fmt.Sprintf("sequence-%s-%03d.cml", source.name, step), layout.data)
				}
			}
			t.Logf("%d edits applied (%d proposals skipped); final language: %+v; applied by kind: %v", step, skipped, language.Counts(), applied)
			if !reduced && len(applied) < len(catalog)-3 {
				t.Errorf("the chain exercised %d of %d edit kinds: %v", len(applied), len(catalog), applied)
			}
		})
	}
}

// languageSize is the number of entries the language records in all.
func languageSize(l vocab.UbiquitousLanguage) int {
	c := l.Counts()
	return c.Contexts + c.Aggregates + c.Entities + c.ValueObjects + c.Invariants + c.Assertions + c.Specifications + c.Events + c.Services + c.Questions + c.Relations
}

// editStepFailures records next over the repository's file, whose layout
// before the edit is given, and returns every property of the step that does
// not hold together with the layout the edit wrote. The file is left as the
// edit wrote it so a caller may continue the chain.
func editStepFailures(t *testing.T, repo Repository, beforeLayout recordingLayout, previous, next vocab.UbiquitousLanguage) ([]string, recordingLayout) {
	t.Helper()
	var failures []string
	fail := func(format string, args ...any) {
		failures = append(failures, "  "+fmt.Sprintf(format, args...))
	}
	if err := repo.Record(next); err != nil {
		fail("Record refused the edit: %v", err)
		return failures, beforeLayout
	}
	after := stateOf(t, repo.Path())
	afterLayout := layoutOf(t, after.data)

	if d := next.Difference(afterLayout.lang); d != "" {
		fail("the file does not read back as the requested language: %s", d)
	}
	validateContextMapperFile(t, afterLayout.file)
	if formatted, err := syntax.FormatChecked(afterLayout.file); err != nil || !bytes.Equal(formatted, after.data) {
		fail("the written file is not a formatter fixed point (err %v)", err)
	}
	if violations := editViolations(beforeLayout, afterLayout); len(violations) > 0 {
		fail("the edit touched lines of elements that did not change:\n    %s", strings.Join(violations, "\n    "))
	}

	backdate(t, repo.Path())
	settled := stateOf(t, repo.Path())
	if err := repo.Record(next); err != nil {
		fail("recording the same language again failed: %v", err)
	} else if message := settled.differsFrom(repo.Path()); message != "" {
		fail("recording the same language a second time rewrote the file: %s", message)
	}

	inverse := syncFixtureRepository(t, string(after.data))
	if err := inverse.Record(previous); err != nil {
		fail("recording the previous language again failed: %v", err)
		return failures, afterLayout
	}
	restored := stateOf(t, inverse.Path()).data
	if bytes.Equal(restored, beforeLayout.data) {
		return failures, afterLayout
	}
	restoredLayout := layoutOf(t, restored)
	if d := previous.Difference(restoredLayout.lang); d != "" {
		fail("the inverse edit does not read back as the previous language: %s", d)
	}
	validateContextMapperFile(t, restoredLayout.file)
	if violations := editViolations(afterLayout, restoredLayout); len(violations) > 0 {
		fail("the inverse edit touched lines of elements that did not change:\n    %s", strings.Join(violations, "\n    "))
	}
	// An element the file no longer holds returns where the adapter places a
	// new element, with the hints and operations in the order it writes them,
	// and the file keeps no record of the place it left or the form it had. The
	// bytes return exactly when the edit removed nothing.
	if !removesElements(beforeLayout, afterLayout) {
		violations, residue := residueViolations(beforeLayout, restoredLayout, scopeOf(beforeLayout, afterLayout))
		switch {
		case len(violations) > 0:
			fail("the inverse edit did not restore the previous bytes:\n    %s", strings.Join(violations, "\n    "))
		case !residue:
			fail("the inverse edit restored every line but not the previous bytes: the blank lines differ")
		}
	}
	return failures, afterLayout
}
