package yamlvocab

import (
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// YAML resolver tags for the node surgery.
const (
	tagStr  = "!!str"
	tagInt  = "!!int"
	tagSeq  = "!!seq"
	tagMap  = "!!map"
	tagNull = "!!null"
)

// foldAt is the length past which prose is written as a folded block
// scalar rather than a plain one-line value; reflowFolded then fills
// the block to lineWidth.
const foldAt = 60

// freshDocument writes the whole language into a new node tree, the
// editor schema modeline above the first key.
func freshDocument(m vocab.UbiquitousLanguage, modeline string) *yaml.Node {
	body := newMapping()
	versionKey := stringScalar(keyVersion)
	versionKey.HeadComment = modeline
	body.Content = []*yaml.Node{versionKey, versionScalar()}
	appendKV(body, keyProject, stringScalar(m.Project))
	if m.Description != "" {
		appendKV(body, keyDescription, textScalar(m.Description))
	}
	appendKV(body, keyContexts, contexts(m).section())
	if len(m.Relations) > 0 {
		appendKV(body, keyRelations, buildRelations(m.Relations))
	}
	return &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{body}}
}

// syncDocument edits an existing node tree until it records m: every
// entry the language still records is updated in place, every entry
// it no longer records is dropped, and every new entry is appended to
// its section in language order.
func syncDocument(body *yaml.Node, m vocab.UbiquitousLanguage) {
	ensureVersion(body)
	setText(body, keyProject, m.Project, documentKeys)
	setText(body, keyDescription, m.Description, documentKeys)
	contexts(m).sync(body, true)
	syncRelations(body, m.Relations)
}

func ensureVersion(body *yaml.Node) {
	existing, idx := findMapEntry(body, keyVersion)
	if idx >= 0 {
		existing.Kind = yaml.ScalarNode
		existing.Tag = tagInt
		existing.Value = strconv.Itoa(vocab.UbiquitousLanguageVersion)
		existing.Content = nil
		return
	}
	body.Content = append([]*yaml.Node{stringScalar(keyVersion), versionScalar()}, body.Content...)
}

// keyed describes one mapping section keyed by name or key: how its
// entries are named, built fresh, and updated in place. The same
// description writes a fresh section and edits an existing one.
type keyed[T any] struct {
	key    string
	order  []string
	items  []T
	name   func(T) string
	build  func(T) *yaml.Node
	update func(*yaml.Node, T)
}

// section builds the mapping fresh.
func (k keyed[T]) section() *yaml.Node {
	m := newMapping()
	for _, it := range k.items {
		appendKV(m, k.name(it), k.build(it))
	}
	return m
}

// appendTo appends the section to a fresh parent when it has entries.
func (k keyed[T]) appendTo(parent *yaml.Node) {
	if len(k.items) > 0 {
		appendKV(parent, k.key, k.section())
	}
}

// sync edits the section under parent: kept entries stay in file order
// with their comments, dropped entries go, new entries are appended in
// language order. An empty section is removed unless required.
func (k keyed[T]) sync(parent *yaml.Node, required bool) {
	existing, idx := findMapEntry(parent, k.key)
	if len(k.items) == 0 && !required {
		removeField(parent, k.key)
		return
	}
	if idx < 0 || existing.Kind != yaml.MappingNode {
		setField(parent, k.key, k.section(), k.order)
		return
	}
	wanted := make(map[string]T, len(k.items))
	for _, it := range k.items {
		wanted[k.name(it)] = it
	}
	seen := make(map[string]bool, len(k.items))
	kept := make([]*yaml.Node, 0, 2*len(k.items))
	for i := 0; i+1 < len(existing.Content); i += 2 {
		key, value := existing.Content[i], existing.Content[i+1]
		it, ok := wanted[key.Value]
		if !ok || seen[key.Value] {
			continue
		}
		k.update(value, it)
		kept = append(kept, key, value)
		seen[key.Value] = true
	}
	for _, it := range k.items {
		if name := k.name(it); !seen[name] {
			kept = append(kept, stringScalar(name), k.build(it))
		}
	}
	existing.Content = kept
	if len(kept) > 0 {
		// An emptied flow mapping ({}) grows back as a block.
		existing.Style = 0
	}
}

func contexts(m vocab.UbiquitousLanguage) keyed[vocab.BoundedContext] {
	return keyed[vocab.BoundedContext]{
		key: keyContexts, order: documentKeys, items: m.Contexts,
		name:   func(c vocab.BoundedContext) string { return c.Name },
		build:  buildContext,
		update: updateContext,
	}
}

func buildContext(c vocab.BoundedContext) *yaml.Node {
	m := newMapping()
	appendKV(m, keyDefinition, textScalar(c.Definition))
	aggregates(c).appendTo(m)
	valueObjects(c).appendTo(m)
	events(c).appendTo(m)
	services(c).appendTo(m)
	specifications(c).appendTo(m)
	questions(c).appendTo(m)
	return m
}

func updateContext(n *yaml.Node, c vocab.BoundedContext) {
	if n.Kind != yaml.MappingNode {
		*n = *buildContext(c)
		return
	}
	setText(n, keyDefinition, c.Definition, contextKeys)
	aggregates(c).sync(n, false)
	valueObjects(c).sync(n, false)
	events(c).sync(n, false)
	services(c).sync(n, false)
	specifications(c).sync(n, false)
	questions(c).sync(n, false)
}

func aggregates(c vocab.BoundedContext) keyed[vocab.Aggregate] {
	return keyed[vocab.Aggregate]{
		key: keyAggregates, order: contextKeys, items: c.Aggregates,
		name:   func(a vocab.Aggregate) string { return a.Name },
		build:  buildAggregate,
		update: updateAggregate,
	}
}

func buildAggregate(a vocab.Aggregate) *yaml.Node {
	m := newMapping()
	appendKV(m, keyDefinition, textScalar(a.Definition))
	appendKV(m, keyIdentity, stringScalar(a.Identity))
	appendList(m, keyAliases, a.Aliases)
	entities(a).appendTo(m)
	invariants(a.Invariants, aggregateKeys).appendTo(m)
	assertions(a).appendTo(m)
	if a.Repository != "" {
		appendKV(m, keyRepository, stringScalar(a.Repository))
	}
	if a.Factory != "" {
		appendKV(m, keyFactory, stringScalar(a.Factory))
	}
	return m
}

func updateAggregate(n *yaml.Node, a vocab.Aggregate) {
	if n.Kind != yaml.MappingNode {
		*n = *buildAggregate(a)
		return
	}
	setText(n, keyDefinition, a.Definition, aggregateKeys)
	setText(n, keyIdentity, a.Identity, aggregateKeys)
	setList(n, keyAliases, a.Aliases, aggregateKeys)
	entities(a).sync(n, false)
	invariants(a.Invariants, aggregateKeys).sync(n, false)
	assertions(a).sync(n, false)
	setText(n, keyRepository, a.Repository, aggregateKeys)
	setText(n, keyFactory, a.Factory, aggregateKeys)
}

func entities(a vocab.Aggregate) keyed[vocab.Entity] {
	return keyed[vocab.Entity]{
		key: keyEntities, order: aggregateKeys, items: a.Entities,
		name: func(e vocab.Entity) string { return e.Name },
		build: func(e vocab.Entity) *yaml.Node {
			m := newMapping()
			appendKV(m, keyDefinition, textScalar(e.Definition))
			if e.Identity != "" {
				appendKV(m, keyIdentity, stringScalar(e.Identity))
			}
			appendList(m, keyAliases, e.Aliases)
			return m
		},
		update: func(n *yaml.Node, e vocab.Entity) {
			if n.Kind != yaml.MappingNode {
				*n = *entities(a).build(e)
				return
			}
			setText(n, keyDefinition, e.Definition, entityKeys)
			setText(n, keyIdentity, e.Identity, entityKeys)
			setList(n, keyAliases, e.Aliases, entityKeys)
		},
	}
}

// invariants describes an owner's keyed statements; order is the
// owner's key order.
func invariants(invs []vocab.Invariant, order []string) keyed[vocab.Invariant] {
	return keyed[vocab.Invariant]{
		key: keyInvariants, order: order, items: invs,
		name:   func(inv vocab.Invariant) string { return inv.Key },
		build:  func(inv vocab.Invariant) *yaml.Node { return textScalar(inv.Statement) },
		update: func(n *yaml.Node, inv vocab.Invariant) { setScalar(n, textScalar(inv.Statement)) },
	}
}

func assertions(a vocab.Aggregate) keyed[vocab.Assertion] {
	return keyed[vocab.Assertion]{
		key: keyAssertions, order: aggregateKeys, items: a.Assertions,
		name: func(as vocab.Assertion) string { return as.Key },
		build: func(as vocab.Assertion) *yaml.Node {
			m := newMapping()
			appendKV(m, keyOn, stringScalar(as.On))
			appendKV(m, keyStatement, textScalar(as.Statement))
			return m
		},
		update: func(n *yaml.Node, as vocab.Assertion) {
			if n.Kind != yaml.MappingNode {
				*n = *assertions(a).build(as)
				return
			}
			setText(n, keyOn, as.On, assertionKeys)
			setText(n, keyStatement, as.Statement, assertionKeys)
		},
	}
}

func valueObjects(c vocab.BoundedContext) keyed[vocab.ValueObject] {
	return keyed[vocab.ValueObject]{
		key: keyValueObjects, order: contextKeys, items: c.ValueObjects,
		name: func(v vocab.ValueObject) string { return v.Name },
		build: func(v vocab.ValueObject) *yaml.Node {
			m := newMapping()
			appendKV(m, keyDefinition, textScalar(v.Definition))
			appendList(m, keyAliases, v.Aliases)
			invariants(v.Invariants, valueObjectKeys).appendTo(m)
			return m
		},
		update: func(n *yaml.Node, v vocab.ValueObject) {
			if n.Kind != yaml.MappingNode {
				*n = *valueObjects(c).build(v)
				return
			}
			setText(n, keyDefinition, v.Definition, valueObjectKeys)
			setList(n, keyAliases, v.Aliases, valueObjectKeys)
			invariants(v.Invariants, valueObjectKeys).sync(n, false)
		},
	}
}

func events(c vocab.BoundedContext) keyed[vocab.DomainEvent] {
	return keyed[vocab.DomainEvent]{
		key: keyEvents, order: contextKeys, items: c.Events,
		name: func(e vocab.DomainEvent) string { return e.Name },
		build: func(e vocab.DomainEvent) *yaml.Node {
			m := newMapping()
			appendKV(m, keyDefinition, textScalar(e.Definition))
			if e.RaisedBy != "" {
				appendKV(m, keyRaisedBy, stringScalar(e.RaisedBy))
			}
			return m
		},
		update: func(n *yaml.Node, e vocab.DomainEvent) {
			if n.Kind != yaml.MappingNode {
				*n = *events(c).build(e)
				return
			}
			setText(n, keyDefinition, e.Definition, eventKeys)
			setText(n, keyRaisedBy, e.RaisedBy, eventKeys)
		},
	}
}

func services(c vocab.BoundedContext) keyed[vocab.DomainService] {
	return keyed[vocab.DomainService]{
		key: keyServices, order: contextKeys, items: c.Services,
		name:   func(s vocab.DomainService) string { return s.Name },
		build:  func(s vocab.DomainService) *yaml.Node { return definitionOnly(s.Definition) },
		update: func(n *yaml.Node, s vocab.DomainService) { updateDefinitionOnly(n, s.Definition, serviceKeys) },
	}
}

func specifications(c vocab.BoundedContext) keyed[vocab.Specification] {
	return keyed[vocab.Specification]{
		key: keySpecifications, order: contextKeys, items: c.Specifications,
		name:   func(s vocab.Specification) string { return s.Name },
		build:  func(s vocab.Specification) *yaml.Node { return definitionOnly(s.Definition) },
		update: func(n *yaml.Node, s vocab.Specification) { updateDefinitionOnly(n, s.Definition, specificationKeys) },
	}
}

func questions(c vocab.BoundedContext) keyed[vocab.Question] {
	return keyed[vocab.Question]{
		key: keyQuestions, order: contextKeys, items: c.Questions,
		name:   func(q vocab.Question) string { return q.Key },
		build:  func(q vocab.Question) *yaml.Node { return textScalar(q.Text) },
		update: func(n *yaml.Node, q vocab.Question) { setScalar(n, textScalar(q.Text)) },
	}
}

func definitionOnly(definition string) *yaml.Node {
	m := newMapping()
	appendKV(m, keyDefinition, textScalar(definition))
	return m
}

func updateDefinitionOnly(n *yaml.Node, definition string, order []string) {
	if n.Kind != yaml.MappingNode {
		*n = *definitionOnly(definition)
		return
	}
	setText(n, keyDefinition, definition, order)
}

func buildRelations(rels []vocab.ContextRelation) *yaml.Node {
	seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: tagSeq}
	for _, r := range rels {
		seq.Content = append(seq.Content, buildRelation(r))
	}
	return seq
}

func buildRelation(r vocab.ContextRelation) *yaml.Node {
	m := newMapping()
	appendKV(m, keyFrom, stringScalar(r.From))
	appendKV(m, keyTo, stringScalar(r.To))
	appendKV(m, keyKind, stringScalar(string(r.Kind)))
	if r.Description != "" {
		appendKV(m, keyDescription, textScalar(r.Description))
	}
	return m
}

func updateRelation(n *yaml.Node, r vocab.ContextRelation) {
	if n.Kind != yaml.MappingNode {
		*n = *buildRelation(r)
		return
	}
	setText(n, keyFrom, r.From, relationKeys)
	setText(n, keyTo, r.To, relationKeys)
	setText(n, keyKind, string(r.Kind), relationKeys)
	setText(n, keyDescription, r.Description, relationKeys)
}

// syncRelations edits the relations sequence, each edge identified by
// its from and to contexts.
func syncRelations(body *yaml.Node, rels []vocab.ContextRelation) {
	existing, idx := findMapEntry(body, keyRelations)
	if len(rels) == 0 {
		removeField(body, keyRelations)
		return
	}
	if idx < 0 || existing.Kind != yaml.SequenceNode {
		setField(body, keyRelations, buildRelations(rels), documentKeys)
		return
	}
	wanted := make(map[string]vocab.ContextRelation, len(rels))
	for _, r := range rels {
		wanted[relationKey(r.From, r.To)] = r
	}
	seen := make(map[string]bool, len(rels))
	kept := make([]*yaml.Node, 0, len(rels))
	for _, item := range existing.Content {
		k := relationKey(mappingText(item, keyFrom), mappingText(item, keyTo))
		r, ok := wanted[k]
		if !ok || seen[k] {
			continue
		}
		updateRelation(item, r)
		kept = append(kept, item)
		seen[k] = true
	}
	for _, r := range rels {
		if k := relationKey(r.From, r.To); !seen[k] {
			kept = append(kept, buildRelation(r))
		}
	}
	existing.Content = kept
	existing.Style = 0
}

func relationKey(from, to string) string {
	return from + "\x00" + to
}

// mappingText reads one scalar of a mapping node, "" when absent.
func mappingText(item *yaml.Node, key string) string {
	if item == nil || item.Kind != yaml.MappingNode {
		return ""
	}
	value, idx := findMapEntry(item, key)
	if idx < 0 || value.Kind != yaml.ScalarNode {
		return ""
	}
	return value.Value
}

// setText sets a text property, removing it when empty.
func setText(mapping *yaml.Node, key, value string, order []string) {
	if value == "" {
		removeField(mapping, key)
		return
	}
	setField(mapping, key, textScalar(value), order)
}

// setList sets a list property, removing it when empty.
func setList(mapping *yaml.Node, key string, values []string, order []string) {
	if len(values) == 0 {
		removeField(mapping, key)
		return
	}
	setField(mapping, key, stringSequence(values), order)
}

func appendList(mapping *yaml.Node, key string, values []string) {
	if len(values) > 0 {
		appendKV(mapping, key, stringSequence(values))
	}
}

// setScalar rewrites a scalar in place, keeping its comments and
// style; a node of another kind is replaced.
func setScalar(existing, value *yaml.Node) {
	if existing.Kind != yaml.ScalarNode {
		*existing = *value
		return
	}
	existing.Tag = value.Tag
	existing.Value = value.Value
}

// setField sets one property. An existing value of the same kind is
// rewritten in place so the comments and style hanging off it survive;
// a value of another kind is replaced; a missing property is inserted
// at its place in the block's key order.
func setField(mapping *yaml.Node, key string, value *yaml.Node, order []string) {
	existing, idx := findMapEntry(mapping, key)
	if idx < 0 {
		insertField(mapping, key, value, order)
		return
	}
	if existing.Kind != value.Kind {
		mapping.Content[idx+1] = value
		return
	}
	existing.Tag = value.Tag
	existing.Value = value.Value
	existing.Content = value.Content
}

// insertField places a new property after the last present property
// that precedes it in the block's key order.
func insertField(mapping *yaml.Node, key string, value *yaml.Node, order []string) {
	rank := indexOf(order, key)
	insertAt := len(mapping.Content)
	if rank >= 0 {
		insertAt = 0
		for i := 0; i+1 < len(mapping.Content); i += 2 {
			if r := indexOf(order, mapping.Content[i].Value); r >= 0 && r < rank {
				insertAt = i + 2
			}
		}
	}
	tail := append([]*yaml.Node{stringScalar(key), value}, mapping.Content[insertAt:]...)
	mapping.Content = append(mapping.Content[:insertAt], tail...)
}

func removeField(mapping *yaml.Node, key string) {
	if _, idx := findMapEntry(mapping, key); idx >= 0 {
		mapping.Content = append(mapping.Content[:idx], mapping.Content[idx+2:]...)
	}
}

func findMapEntry(mapping *yaml.Node, key string) (*yaml.Node, int) {
	if mapping == nil {
		return nil, -1
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i] != nil && mapping.Content[i].Value == key {
			return mapping.Content[i+1], i
		}
	}
	return nil, -1
}

func indexOf(order []string, key string) int {
	for i, k := range order {
		if k == key {
			return i
		}
	}
	return -1
}

func appendKV(mapping *yaml.Node, key string, value *yaml.Node) {
	mapping.Content = append(mapping.Content, stringScalar(key), value)
}

func newMapping() *yaml.Node {
	return &yaml.Node{Kind: yaml.MappingNode, Tag: tagMap}
}

func stringScalar(v string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: tagStr, Value: v}
}

func versionScalar() *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: tagInt, Value: strconv.Itoa(vocab.UbiquitousLanguageVersion)}
}

// textScalar builds a scalar for prose: a folded block once the text
// is long or spans paragraphs, so the file reads as the authors write
// it and no punctuation forces quoting.
func textScalar(v string) *yaml.Node {
	n := stringScalar(v)
	if len(v) > foldAt || strings.Contains(v, "\n") {
		n.Style = yaml.FoldedStyle
	}
	return n
}

// stringSequence builds a flow list ([a, b]) for aliases.
func stringSequence(values []string) *yaml.Node {
	seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: tagSeq, Style: yaml.FlowStyle}
	for _, v := range values {
		seq.Content = append(seq.Content, stringScalar(v))
	}
	return seq
}
