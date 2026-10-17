package cmlvocab

import (
	"fmt"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/vocab"
	"github.com/wixregiga/arclint/internal/infrastructure/vocab/cml/syntax"
)

// Render creates a complete CML recording of a validated Ubiquitous Language.
func Render(lang vocab.UbiquitousLanguage) ([]byte, error) {
	if _, err := vocab.NewUbiquitousLanguage(lang.Project, lang.Description, lang.Contexts, lang.Relations); err != nil {
		return nil, fmt.Errorf("render CML: %w", err)
	}
	file := freshFile(lang)
	data, err := syntax.FormatChecked(file)
	if err != nil {
		return nil, fmt.Errorf("validate fresh CML: %w", err)
	}
	recording, _ := vocab.NewDomainRecording("domain.cml")
	read, err := Parse(data, recording)
	if err != nil {
		return nil, fmt.Errorf("verify rendered CML: %w", err)
	}
	if !sameLanguage(lang, read) {
		return nil, fmt.Errorf("rendered CML does not preserve the recorded language")
	}
	return data, nil
}

type writer struct {
	names           *nameAllocator
	aggregateNames  map[string]int
	repositoryNames map[string]int
}

func freshFile(lang vocab.UbiquitousLanguage) *syntax.File { return freshBoundFile(lang, nil) }

func freshBoundFile(lang vocab.UbiquitousLanguage, existing *projection) *syntax.File {
	w := &writer{names: newNames(), aggregateNames: map[string]int{}, repositoryNames: repositoryNameCounts(lang, existing)}
	if existing != nil {
		reserveExistingNames(w.names, existing.file)
		for name, bc := range existing.contexts {
			w.names.names["context\x00"+name] = bc.Name
		}
		for context, names := range existing.names {
			for handle, exact := range names {
				w.names.names["term:"+context+"\x00"+exact] = handle
			}
		}
		for id, service := range existing.factories {
			context, _, _ := strings.Cut(id, "\x00")
			w.names.names["term:"+context+"\x00"+exactName(service.Name, &service.Node)] = service.Name
		}
		if len(existing.file.Domains) > 0 {
			d := existing.file.Domains[0]
			if exactName(d.Name, &d.Node) == lang.Project {
				w.names.names["project\x00"+lang.Project] = d.Name
			}
		}
	}
	w.names.reserve(lang.Project)
	for _, c := range lang.Contexts {
		w.names.reserve(c.Name)
		for _, term := range c.Terms() {
			w.names.reserve(term.Name)
		}
		for _, a := range c.Aggregates {
			w.aggregateNames[a.Name]++
			w.names.reserve(a.Factory)
			w.names.reserve(a.Repository)
		}
	}
	projectName := w.names.name("project", lang.Project)
	file := &syntax.File{Domains: []*syntax.Domain{{Node: namedNode(projectName, lang.Project), Name: projectName, DomainVisionStatement: lang.Description}}}
	file.LeadingComments = []syntax.Comment{{Text: "// The Ubiquitous Language of " + strings.NewReplacer("\r\n", " ", "\r", " ", "\n", " ", "\u0085", " ", "\u2028", " ", "\u2029", " ").Replace(lang.Project) + ", recorded by ArcLint."}}
	contextHandles := map[string]string{}
	for _, c := range lang.Contexts {
		contextHandles[c.Name] = w.names.name("context", c.Name)
	}
	if len(lang.Contexts) > 0 {
		file.Map = &syntax.ContextMap{}
		for _, c := range lang.Contexts {
			file.Map.BoundedContexts = append(file.Map.BoundedContexts, syntax.Ref{Name: contextHandles[c.Name]})
		}
		for _, r := range lang.Relations {
			if r.Kind == vocab.RelationSeparateWays {
				file.Map.TrailingComments = append(file.Map.TrailingComments, directive(separateWaysDirective, r.From, r.To, r.Description))
				continue
			}
			relation := freshRelation(r, contextHandles)
			_, _, _, node := relationshipValue(relation)
			// Attach preceding separate-ways carriers to the next relationship to keep
			// every relation in recorded order, including alternating representations.
			node.LeadingComments = append(node.LeadingComments, file.Map.TrailingComments...)
			file.Map.TrailingComments = nil
			if r.Description != "" {
				node.LeadingComments = append(node.LeadingComments, directive("description", r.Description))
			}
			file.Map.Relationships = append(file.Map.Relationships, relation)
		}
	}
	for _, c := range lang.Contexts {
		bc := w.context(c, contextHandles[c.Name])
		placeContextServices(bc)
		file.BoundedContexts = append(file.BoundedContexts, bc)
	}
	return file
}
func (w *writer) term(context, name string) string { return w.names.name("term:"+context, name) }
func (w *writer) node(context, name string) syntax.Node {
	return namedNode(w.term(context, name), name)
}

func (w *writer) identity(context, name string) *syntax.Attribute {
	if name == "" {
		return nil
	}
	handle := w.term(context, name)
	node := namedNode(handle, name)
	if spelling := cmlID(handle); spelling != handle {
		node.Spellings = map[string]string{"type": spelling}
	}
	return &syntax.Attribute{Node: node, Type: handle, Name: "id", Key: true}
}

func (w *writer) context(c vocab.BoundedContext, handle string) *syntax.BoundedContext {
	bc := &syntax.BoundedContext{Node: namedNode(handle, c.Name), Name: handle, DomainVisionStatement: c.Definition}
	// Allocate every term first: identity references reuse the declaration handle.
	for _, t := range c.Terms() {
		w.term(c.Name, t.Name)
	}
	for _, a := range c.Aggregates {
		container := w.term(c.Name, a.Name)
		if w.aggregateNames[a.Name] > 1 {
			candidate := c.Name + "_" + a.Name
			// Container names are separate from root names and globally unique.
			container = w.names.unique("container:"+c.Name, candidate)
		}
		aggregate := &syntax.Aggregate{Name: container}
		root := &syntax.Entity{Node: w.node(c.Name, a.Name), Name: w.term(c.Name, a.Name), Doc: a.Definition, AggregateRoot: true, Hint: replaceHint("", "alias", a.Aliases)}
		root.Attributes = []*syntax.Attribute{w.identity(c.Name, a.Identity)}
		root.Operations = freshOperations(a.Invariants, a.Assertions)
		if a.Repository != "" {
			if identifier.MatchString(a.Repository) && strings.HasSuffix(a.Repository, "Repository") && w.repositoryNames[a.Repository] == 1 {
				root.Repository = &syntax.Repository{Name: a.Repository}
			} else {
				root.Hint = scalarHint(root.Hint, "repository", a.Repository)
			}
		}
		aggregate.DomainObjects = append(aggregate.DomainObjects, root)
		for _, e := range a.Entities {
			member := &syntax.Entity{Node: w.node(c.Name, e.Name), Name: w.term(c.Name, e.Name), Doc: e.Definition, Hint: replaceHint("", "alias", e.Aliases)}
			if e.Identity != "" {
				member.Attributes = append(member.Attributes, w.identity(c.Name, e.Identity))
			}
			aggregate.DomainObjects = append(aggregate.DomainObjects, member)
		}
		if a.Factory != "" {
			aggregate.Services = append(aggregate.Services, &syntax.Service{Node: w.node(c.Name, a.Factory), Name: w.term(c.Name, a.Factory), Hint: "arclint.role=factory"})
		}
		bc.Aggregates = append(bc.Aggregates, aggregate)
	}
	if len(c.ValueObjects)+len(c.Specifications)+len(c.Events) > 0 {
		module := &syntax.SculptorModule{Name: handle}
		for _, v := range c.ValueObjects {
			module.DomainObjects = append(module.DomainObjects, &syntax.ValueObject{Node: w.node(c.Name, v.Name), Name: w.term(c.Name, v.Name), Doc: v.Definition, Hint: replaceHint("", "alias", v.Aliases), Operations: freshOperations(v.Invariants, nil)})
		}
		for _, s := range c.Specifications {
			module.DomainObjects = append(module.DomainObjects, &syntax.ValueObject{Node: w.node(c.Name, s.Name), Name: w.term(c.Name, s.Name), Doc: s.Definition, Operations: []*syntax.DomainObjectOperation{{Name: satisfactionOperation, ReturnType: &syntax.ComplexType{Type: "boolean"}}}})
		}
		for _, e := range c.Events {
			event := &syntax.DomainEvent{Node: w.node(c.Name, e.Name), Name: w.term(c.Name, e.Name), Doc: e.Definition}
			if e.RaisedBy != "" {
				event.BelongsTo = syntax.Ref{Name: w.term(c.Name, e.RaisedBy)}
			}
			module.DomainObjects = append(module.DomainObjects, event)
		}
		bc.Modules = append(bc.Modules, module)
	}
	for _, s := range c.Services {
		bc.DomainServices = append(bc.DomainServices, &syntax.Service{Node: w.node(c.Name, s.Name), Name: w.term(c.Name, s.Name), Doc: s.Definition, Operations: freshServiceOperations(s.Assertions)})
	}
	for _, q := range c.Questions {
		bc.TrailingComments = append(bc.TrailingComments, directive(questionDirective, q.Key, q.Text))
	}
	return bc
}

func freshOperations(invariants []vocab.Invariant, assertions []vocab.Assertion) []*syntax.DomainObjectOperation {
	var ops []*syntax.DomainObjectOperation
	declared := map[string]bool{}
	for _, inv := range invariants {
		name := invariantPrefix + keyCase(inv.Key)
		ops = append(ops, &syntax.DomainObjectOperation{Doc: inv.Statement, Name: name, Hint: replaceHint("", verifiedByHint, inv.VerifiedBy)})
		declared[name] = true
	}
	for _, a := range assertions {
		if identifier.MatchString(a.On) && !declared[a.On] {
			ops = append(ops, &syntax.DomainObjectOperation{Name: a.On})
			declared[a.On] = true
		}
		name := assertionPrefix + keyCase(a.Key)
		ops = append(ops, &syntax.DomainObjectOperation{Name: name, Doc: a.Statement, Hint: assertionHint(a)})
		declared[name] = true
	}
	return ops
}

func freshServiceOperations(assertions []vocab.Assertion) []*syntax.ServiceOperation {
	var out []*syntax.ServiceOperation
	declared := map[string]bool{}
	for _, a := range assertions {
		if identifier.MatchString(a.On) && !declared[a.On] {
			out = append(out, &syntax.ServiceOperation{Name: a.On})
			declared[a.On] = true
		}
		name := assertionPrefix + keyCase(a.Key)
		out = append(out, &syntax.ServiceOperation{Name: name, Doc: a.Statement, Hint: assertionHint(a)})
		declared[name] = true
	}
	return out
}

// assertionHint carries what CML cannot say of an assertion: the
// operation it constrains, then the tests that verify it.
func assertionHint(a vocab.Assertion) string {
	return replaceHint(scalarHint("", "on", a.On), verifiedByHint, a.VerifiedBy)
}

func freshRelation(r vocab.ContextRelation, contexts map[string]string) syntax.Relationship {
	from, to := syntax.Ref{Name: contexts[r.From]}, syntax.Ref{Name: contexts[r.To]}
	switch r.Kind {
	case vocab.RelationPartnership:
		return &syntax.Partnership{Participant1: from, Participant2: to, Notation: 0}
	case vocab.RelationSharedKernel:
		return &syntax.SharedKernel{Participant1: from, Participant2: to, Notation: 0}
	case vocab.RelationCustomerSupplier:
		return &syntax.CustomerSupplierRelationship{Upstream: from, Downstream: to, Notation: 0}
	default:
		relation := &syntax.UpstreamDownstreamRelationship{Upstream: from, Downstream: to, Notation: 0}
		switch r.Kind {
		case vocab.RelationAnticorruptionLayer:
			relation.DownstreamRoles = []syntax.DownstreamRole{"ACL"}
		case vocab.RelationOpenHostService:
			relation.UpstreamRoles = []syntax.UpstreamRole{"OHS"}
		case vocab.RelationPublishedLanguage:
			relation.UpstreamRoles = []syntax.UpstreamRole{"PL"}
		default:
			relation.DownstreamRoles = []syntax.DownstreamRole{"CF"}
		}
		return relation
	}
}

func cmlID(name string) string {
	if syntax.IsKeyword(name) {
		return "^" + name
	}
	return name
}
