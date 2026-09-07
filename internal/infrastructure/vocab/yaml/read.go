package yamlvocab

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// The keys of the domain file, as the meta-model records them.
const (
	keyVersion        = "version"
	keyProject        = "project"
	keyDescription    = "description"
	keyContexts       = "contexts"
	keyRelations      = "relations"
	keyDefinition     = "definition"
	keyAggregates     = "aggregates"
	keyValueObjects   = "value_objects"
	keyEvents         = "events"
	keyServices       = "services"
	keySpecifications = "specifications"
	keyQuestions      = "questions"
	keyIdentity       = "identity"
	keyAliases        = "aliases"
	keyEntities       = "entities"
	keyInvariants     = "invariants"
	keyAssertions     = "assertions"
	keyRepository     = "repository"
	keyFactory        = "factory"
	keyOn             = "on"
	keyStatement      = "statement"
	keyRaisedBy       = "raised_by"
	keyFrom           = "from"
	keyTo             = "to"
	keyKind           = "kind"
)

// The keys each building block's entry may carry, in the meta-model's
// order, less the one the entry is keyed by in the file. A test holds
// each list to the meta-model's recording so the adapter cannot accept
// a key the meta-model does not record, nor drop one it does.
var (
	documentKeys      = []string{keyVersion, keyProject, keyDescription, keyContexts, keyRelations}
	contextKeys       = []string{keyDefinition, keyAggregates, keyValueObjects, keyEvents, keyServices, keySpecifications, keyQuestions}
	aggregateKeys     = []string{keyDefinition, keyIdentity, keyAliases, keyEntities, keyInvariants, keyAssertions, keyRepository, keyFactory}
	entityKeys        = []string{keyDefinition, keyIdentity, keyAliases}
	valueObjectKeys   = []string{keyDefinition, keyAliases, keyInvariants}
	assertionKeys     = []string{keyOn, keyStatement}
	eventKeys         = []string{keyDefinition, keyRaisedBy}
	serviceKeys       = []string{keyDefinition}
	specificationKeys = []string{keyDefinition}
	relationKeys      = []string{keyFrom, keyTo, keyKind, keyDescription}
)

// source reads one domain file as a node tree, so every entry keeps
// the line it is written on and every key is held to what its
// building block records. label names the file in messages.
type source struct {
	label string
}

// parse decodes and validates domain file content.
func parse(data []byte, label string) (vocab.UbiquitousLanguage, error) {
	s := source{label: label}
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return vocab.UbiquitousLanguage{}, fmt.Errorf("%s: %v", label, err)
	}
	body := documentBody(&root)
	if body == nil {
		return vocab.UbiquitousLanguage{}, fmt.Errorf("%s: the file is empty; it records at least version, project, and contexts", label)
	}
	return s.language(body)
}

// documentBody returns the node under the document, nil for an empty
// document (no content at all, or comments only).
func documentBody(root *yaml.Node) *yaml.Node {
	switch root.Kind {
	case 0:
		return nil
	case yaml.DocumentNode:
		if len(root.Content) == 0 {
			return nil
		}
		return root.Content[0]
	default:
		return root
	}
}

func (s source) errorf(line int, format string, args ...any) error {
	return fmt.Errorf("%s: line %d: %s", s.label, line, fmt.Sprintf(format, args...))
}

// entry is one key/value pair of a mapping with the line of its key.
type entry struct {
	key   string
	line  int
	value *yaml.Node
}

// aliasHopLimit bounds alias resolution so a self-referencing chain
// cannot spin.
const aliasHopLimit = 64

// resolve follows an alias to the node it names.
func resolve(n *yaml.Node) *yaml.Node {
	for hops := 0; n != nil && n.Kind == yaml.AliasNode && hops < aliasHopLimit; hops++ {
		n = n.Alias
	}
	return n
}

// isNull reports a key written without a value. The schema types every
// property, so a bare key is an error here as it is in the editor: the
// author either records the section or leaves the key out.
func isNull(n *yaml.Node) bool {
	return n == nil || (n.Kind == yaml.ScalarNode && n.Tag == tagNull)
}

// lineOf is the line a node is written on, 0 when there is no node.
func lineOf(n *yaml.Node) int {
	if n = resolve(n); n == nil {
		return 0
	}
	return n.Line
}

// mapping reads a mapping node's entries in file order. keys lists what
// the block records; nil accepts any key, for the sections keyed by
// name. A key written twice is an error: the file would otherwise
// record one meaning and show another.
func (s source) mapping(n *yaml.Node, what string, keys []string) ([]entry, error) {
	n = resolve(n)
	if isNull(n) {
		return nil, s.errorf(lineOf(n), "%s has no value", what)
	}
	if n.Kind != yaml.MappingNode {
		return nil, s.errorf(n.Line, "%s is not a mapping", what)
	}
	seen := map[string]int{}
	out := make([]entry, 0, len(n.Content)/2)
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		if k.Kind != yaml.ScalarNode {
			return nil, s.errorf(k.Line, "%s: a key is not text", what)
		}
		if keys != nil && !slices.Contains(keys, k.Value) {
			return nil, s.errorf(k.Line, "%s: unknown key %q; it records %s", what, k.Value, strings.Join(keys, ", "))
		}
		if prev, dup := seen[k.Value]; dup {
			return nil, s.errorf(k.Line, "%s: %q is written twice (also line %d)", what, k.Value, prev)
		}
		seen[k.Value] = k.Line
		out = append(out, entry{key: k.Value, line: k.Line, value: v})
	}
	return out, nil
}

// fields is one block's entries with typed reads. The first failure
// is kept and every later read is a no-op, so a block reader stays a
// straight list of the properties it records.
type fields struct {
	s       source
	what    string
	entries []entry
	err     error
}

func (s source) fields(n *yaml.Node, what string, keys []string) *fields {
	entries, err := s.mapping(n, what, keys)
	return &fields{s: s, what: what, entries: entries, err: err}
}

func (f *fields) fail(err error) {
	if f.err == nil {
		f.err = err
	}
}

func (f *fields) node(key string) (entry, bool) {
	for _, e := range f.entries {
		if e.key == key {
			return e, true
		}
	}
	return entry{}, false
}

// text reads one text property; absent or null reads as "".
func (f *fields) text(key string) string {
	if f.err != nil {
		return ""
	}
	e, ok := f.node(key)
	if !ok {
		return ""
	}
	value, err := f.s.text(e.value, e.line, f.what+": "+key)
	f.fail(err)
	return value
}

// text reads a scalar node as text.
func (s source) text(n *yaml.Node, line int, what string) (string, error) {
	n = resolve(n)
	if isNull(n) {
		return "", s.errorf(line, "%s has no value", what)
	}
	if n.Kind != yaml.ScalarNode {
		return "", s.errorf(line, "%s is not text", what)
	}
	return n.Value, nil
}

// list reads a sequence of text (aliases); absent reads as nil.
func (f *fields) list(key string) []string {
	if f.err != nil {
		return nil
	}
	e, ok := f.node(key)
	if !ok {
		return nil
	}
	n := resolve(e.value)
	if isNull(n) {
		f.fail(f.s.errorf(e.line, "%s: %s has no value", f.what, key))
		return nil
	}
	if n.Kind != yaml.SequenceNode {
		f.fail(f.s.errorf(e.line, "%s: %s is not a list", f.what, key))
		return nil
	}
	out := make([]string, 0, len(n.Content))
	for i, item := range n.Content {
		value, err := f.s.text(item, item.Line, fmt.Sprintf("%s: %s %d", f.what, key, i+1))
		if err != nil {
			f.fail(err)
			return nil
		}
		out = append(out, value)
	}
	return out
}

// keyed reads a section keyed by name or key: its entries in file
// order.
func (f *fields) keyed(key string) []entry {
	if f.err != nil {
		return nil
	}
	e, ok := f.node(key)
	if !ok {
		return nil
	}
	entries, err := f.s.mapping(e.value, f.what+": "+key, nil)
	f.fail(err)
	return entries
}

// items reads a sequence of nodes (relations); absent reads as nil.
func (f *fields) items(key string) []*yaml.Node {
	if f.err != nil {
		return nil
	}
	e, ok := f.node(key)
	if !ok {
		return nil
	}
	n := resolve(e.value)
	if isNull(n) {
		f.fail(f.s.errorf(e.line, "%s: %s has no value", f.what, key))
		return nil
	}
	if n.Kind != yaml.SequenceNode {
		f.fail(f.s.errorf(e.line, "%s: %s is not a list", f.what, key))
		return nil
	}
	return n.Content
}

// language reads the document.
func (s source) language(body *yaml.Node) (vocab.UbiquitousLanguage, error) {
	f := s.fields(body, "the domain file", documentKeys)
	if f.err != nil {
		return vocab.UbiquitousLanguage{}, f.err
	}
	if err := s.version(f); err != nil {
		return vocab.UbiquitousLanguage{}, err
	}
	project := f.text(keyProject)
	description := f.text(keyDescription)
	if _, ok := f.node(keyContexts); !ok {
		return vocab.UbiquitousLanguage{}, fmt.Errorf("%s: contexts is missing (contexts: {} records none yet)", s.label)
	}
	var contexts []vocab.BoundedContext
	for _, e := range f.keyed(keyContexts) {
		c, err := s.context(e)
		if err != nil {
			return vocab.UbiquitousLanguage{}, err
		}
		contexts = append(contexts, c)
	}
	var relations []vocab.ContextRelation
	for _, n := range f.items(keyRelations) {
		r, err := s.relation(n)
		if err != nil {
			return vocab.UbiquitousLanguage{}, err
		}
		relations = append(relations, r)
	}
	if f.err != nil {
		return vocab.UbiquitousLanguage{}, f.err
	}
	lang, err := vocab.NewUbiquitousLanguage(project, description, contexts, relations)
	if err != nil {
		return vocab.UbiquitousLanguage{}, fmt.Errorf("%s: %w", s.label, err)
	}
	return lang, nil
}

// version applies the one document version this arclint reads.
func (s source) version(f *fields) error {
	e, ok := f.node(keyVersion)
	if !ok {
		return fmt.Errorf("%s: version is missing (this arclint accepts version %d)", s.label, vocab.UbiquitousLanguageVersion)
	}
	raw, err := s.text(e.value, e.line, "version")
	if err != nil {
		return err
	}
	version, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || version != vocab.UbiquitousLanguageVersion {
		return s.errorf(e.line, "unsupported version %q (this arclint accepts version %d)", raw, vocab.UbiquitousLanguageVersion)
	}
	return nil
}

// context reads one bounded context, keyed by its name.
func (s source) context(e entry) (vocab.BoundedContext, error) {
	f := s.fields(e.value, fmt.Sprintf("context %q", e.key), contextKeys)
	c := vocab.BoundedContext{Name: e.key, Line: e.line}
	c.Definition = f.text(keyDefinition)
	for _, m := range f.keyed(keyAggregates) {
		a, err := s.aggregate(f.what, m)
		if err != nil {
			return vocab.BoundedContext{}, err
		}
		c.Aggregates = append(c.Aggregates, a)
	}
	for _, m := range f.keyed(keyValueObjects) {
		v, err := s.valueObject(f.what, m)
		if err != nil {
			return vocab.BoundedContext{}, err
		}
		c.ValueObjects = append(c.ValueObjects, v)
	}
	for _, m := range f.keyed(keyEvents) {
		g := s.fields(m.value, fmt.Sprintf("%s: event %q", f.what, m.key), eventKeys)
		c.Events = append(c.Events, vocab.DomainEvent{
			Name: m.key, Definition: g.text(keyDefinition), RaisedBy: g.text(keyRaisedBy), Line: m.line,
		})
		if g.err != nil {
			return vocab.BoundedContext{}, g.err
		}
	}
	for _, m := range f.keyed(keyServices) {
		g := s.fields(m.value, fmt.Sprintf("%s: service %q", f.what, m.key), serviceKeys)
		c.Services = append(c.Services, vocab.DomainService{Name: m.key, Definition: g.text(keyDefinition), Line: m.line})
		if g.err != nil {
			return vocab.BoundedContext{}, g.err
		}
	}
	for _, m := range f.keyed(keySpecifications) {
		g := s.fields(m.value, fmt.Sprintf("%s: specification %q", f.what, m.key), specificationKeys)
		c.Specifications = append(c.Specifications, vocab.Specification{Name: m.key, Definition: g.text(keyDefinition), Line: m.line})
		if g.err != nil {
			return vocab.BoundedContext{}, g.err
		}
	}
	for _, m := range f.keyed(keyQuestions) {
		text, err := s.text(m.value, m.line, fmt.Sprintf("%s: question %q", f.what, m.key))
		if err != nil {
			return vocab.BoundedContext{}, err
		}
		c.Questions = append(c.Questions, vocab.Question{Key: m.key, Text: text, Line: m.line})
	}
	return c, f.err
}

// aggregate reads one aggregate with the members, invariants, and
// assertions it owns.
func (s source) aggregate(context string, e entry) (vocab.Aggregate, error) {
	f := s.fields(e.value, fmt.Sprintf("%s: aggregate %q", context, e.key), aggregateKeys)
	a := vocab.Aggregate{Name: e.key, Line: e.line}
	a.Definition = f.text(keyDefinition)
	a.Identity = f.text(keyIdentity)
	a.Aliases = f.list(keyAliases)
	for _, m := range f.keyed(keyEntities) {
		g := s.fields(m.value, fmt.Sprintf("%s: entity %q", f.what, m.key), entityKeys)
		a.Entities = append(a.Entities, vocab.Entity{
			Name: m.key, Definition: g.text(keyDefinition), Identity: g.text(keyIdentity), Aliases: g.list(keyAliases), Line: m.line,
		})
		if g.err != nil {
			return vocab.Aggregate{}, g.err
		}
	}
	invariants, err := s.invariants(f)
	if err != nil {
		return vocab.Aggregate{}, err
	}
	a.Invariants = invariants
	for _, m := range f.keyed(keyAssertions) {
		g := s.fields(m.value, fmt.Sprintf("%s: assertion %q", f.what, m.key), assertionKeys)
		a.Assertions = append(a.Assertions, vocab.Assertion{
			Key: m.key, On: g.text(keyOn), Statement: g.text(keyStatement), Line: m.line,
		})
		if g.err != nil {
			return vocab.Aggregate{}, g.err
		}
	}
	a.Repository = f.text(keyRepository)
	a.Factory = f.text(keyFactory)
	return a, f.err
}

// valueObject reads one value object with the invariants it owns.
func (s source) valueObject(context string, e entry) (vocab.ValueObject, error) {
	f := s.fields(e.value, fmt.Sprintf("%s: value object %q", context, e.key), valueObjectKeys)
	v := vocab.ValueObject{Name: e.key, Line: e.line}
	v.Definition = f.text(keyDefinition)
	v.Aliases = f.list(keyAliases)
	invariants, err := s.invariants(f)
	if err != nil {
		return vocab.ValueObject{}, err
	}
	v.Invariants = invariants
	return v, f.err
}

// invariants reads an owner's keyed statements.
func (s source) invariants(f *fields) ([]vocab.Invariant, error) {
	var out []vocab.Invariant
	for _, m := range f.keyed(keyInvariants) {
		statement, err := s.text(m.value, m.line, fmt.Sprintf("%s: invariant %q", f.what, m.key))
		if err != nil {
			return nil, err
		}
		out = append(out, vocab.Invariant{Key: m.key, Statement: statement, Line: m.line})
	}
	return out, f.err
}

// relation reads one context-map edge.
func (s source) relation(n *yaml.Node) (vocab.ContextRelation, error) {
	f := s.fields(n, "relation", relationKeys)
	r := vocab.ContextRelation{Line: resolve(n).Line}
	r.From = f.text(keyFrom)
	r.To = f.text(keyTo)
	r.Kind = vocab.RelationKind(f.text(keyKind))
	r.Description = f.text(keyDescription)
	return r, f.err
}
