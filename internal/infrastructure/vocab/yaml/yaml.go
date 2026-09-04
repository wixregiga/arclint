// Package yamlvocab loads and stores the project's recorded
// Ubiquitous Language vocabulary from ubiquitous-language.yaml.
// RecordedLanguage is strict (unknown keys are errors). Record
// preserves human authoring of untouched entries (comments and
// ordering) and writes atomically.
package yamlvocab

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

const (
	// localSchemaRel is the in-repo schema path used in the modeline
	// when that file exists under the repository root.
	localSchemaRel = ".agents/skills/domain-librarian/library.schema.json"
	// remoteSchemaURL is the published $id used when the local schema
	// file is absent under the bound root.
	remoteSchemaURL = "https://raw.githubusercontent.com/wixregiga/arclint/main/.agents/skills/domain-librarian/library.schema.json"
)

// Repository implements the domain-owned vocab.Repository port over
// one ubiquitous-language.yaml beside the resolved ruleset root.
type Repository struct {
	path string
	root string
}

// NewRepository binds the repository to filepath.Join(root, UbiquitousLanguageFileName).
func NewRepository(root string) (Repository, error) {
	if root == "" {
		return Repository{}, fmt.Errorf("ubiquitous-language root: empty path")
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return Repository{}, fmt.Errorf("ubiquitous-language root: %w", err)
	}
	abs, err := filepath.Abs(filepath.Join(absRoot, vocab.UbiquitousLanguageFileName))
	if err != nil {
		return Repository{}, fmt.Errorf("ubiquitous-language path: %w", err)
	}
	return Repository{path: abs, root: absRoot}, nil
}

// Path returns the absolute path of the ubiquitous-language.yaml file.
func (r Repository) Path() string { return r.path }

// RecordedLanguage returns the vocabulary and whether the file exists.
// A missing file is (zero UbiquitousLanguage, false, nil). A file that
// cannot become a valid UbiquitousLanguage is an error, never a partial
// value.
func (r Repository) RecordedLanguage() (vocab.UbiquitousLanguage, bool, error) {
	data, err := os.ReadFile(r.path)
	if err != nil {
		if os.IsNotExist(err) {
			return vocab.UbiquitousLanguage{}, false, nil
		}
		return vocab.UbiquitousLanguage{}, false, fmt.Errorf("read %s: %w", r.path, err)
	}
	lang, err := parse(data, r.path)
	if err != nil {
		return vocab.UbiquitousLanguage{}, true, err
	}
	return lang, true, nil
}

// Parse turns authored ubiquitous-language.yaml content into the
// recorded vocabulary, applying the same strict decoding and domain
// invariants as RecordedLanguage. Callers that hold fixture bytes
// rather than a repository file, such as the rule-test harness
// feeding ctx.domain(), parse through here.
func Parse(data []byte) (vocab.UbiquitousLanguage, error) {
	return parse(data, vocab.UbiquitousLanguageFileName)
}

// Parser adapts Parse to the application's VocabularySource port.
type Parser struct{}

// ParseUbiquitousLanguage implements application.VocabularySource.
func (Parser) ParseUbiquitousLanguage(content []byte) (vocab.UbiquitousLanguage, error) {
	return Parse(content)
}

// parse decodes and validates vocabulary content; label names the
// source in error messages.
func parse(data []byte, label string) (vocab.UbiquitousLanguage, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	var doc documentDoc
	if err := decoder.Decode(&doc); err != nil {
		return vocab.UbiquitousLanguage{}, fmt.Errorf("%s: %v", label, err)
	}
	// A second pass over the same bytes, as a node tree, so every
	// recorded entry keeps the line it is written on. The strict
	// decode above stays the one that rejects unknown keys: decoding
	// through yaml.Node would drop that strictness.
	lines := readEntryLines(data)

	if doc.Version != vocab.UbiquitousLanguageVersion {
		version := fmt.Sprintf("%d", doc.Version)
		if doc.Version == 0 {
			version = "missing version"
		}
		return vocab.UbiquitousLanguage{}, fmt.Errorf(
			"%s: unsupported version %s (this arclint accepts version %d)",
			vocab.UbiquitousLanguageFileName, version, vocab.UbiquitousLanguageVersion,
		)
	}

	contexts := make([]vocab.BoundedContext, len(doc.Contexts))
	for i, c := range doc.Contexts {
		at := lines.context(i)
		entities := make([]vocab.Entity, len(c.Entities))
		for j, e := range c.Entities {
			entities[j] = vocab.Entity{
				Definition: vocab.Definition{
					Name:       e.Name,
					Definition: e.Definition,
					Aliases:    e.Aliases,
					Line:       lineAt(at.entities, j),
				},
				Aggregate: e.Aggregate,
			}
		}
		invariants := make([]vocab.Invariant, len(c.Invariants))
		for j, inv := range c.Invariants {
			invariants[j] = vocab.Invariant{
				Statement: inv.Statement,
				Owner:     inv.Owner,
				ID:        inv.ID,
				Line:      lineAt(at.invariants, j),
			}
		}
		assertions := make([]vocab.Assertion, len(c.Assertions))
		for j, a := range c.Assertions {
			assertions[j] = vocab.Assertion{
				Statement: a.Statement,
				Owner:     a.Owner,
				ID:        a.ID,
				On:        a.On,
				Line:      lineAt(at.assertions, j),
			}
		}
		specifications := make([]vocab.Specification, len(c.Specifications))
		for j, s := range c.Specifications {
			specifications[j] = vocab.Specification{
				Name:       s.Name,
				Definition: s.Definition,
				Line:       lineAt(at.specifications, j),
			}
		}
		contexts[i] = vocab.BoundedContext{
			Name:           c.Name,
			Entities:       entities,
			ValueObjects:   toDefs(c.ValueObjects, at.valueObjects),
			Invariants:     invariants,
			Assertions:     assertions,
			Specifications: specifications,
			Events:         toEventDefs(c.Events, at.events),
			Line:           at.line,
		}
	}

	relations := make([]vocab.ContextRelation, len(doc.Relations))
	for i, rel := range doc.Relations {
		kind, err := vocab.ParseRelationKind(rel.Kind)
		if err != nil {
			return vocab.UbiquitousLanguage{}, fmt.Errorf("%s: %w", label, err)
		}
		relations[i] = vocab.ContextRelation{
			From: rel.From,
			To:   rel.To,
			Kind: kind,
			Line: lineAt(lines.relations, i),
		}
	}

	lang, err := vocab.NewUbiquitousLanguage(contexts, relations)
	if err != nil {
		return vocab.UbiquitousLanguage{}, fmt.Errorf("%s: %w", label, err)
	}
	return lang, nil
}

// Record persists the complete vocabulary, preserving human authoring
// (comments, ordering) of untouched entries and writing atomically.
func (r Repository) Record(m vocab.UbiquitousLanguage) error {
	var root yaml.Node
	existing, err := os.ReadFile(r.path)
	switch {
	case err == nil:
		if err := yaml.Unmarshal(existing, &root); err != nil {
			return fmt.Errorf("%s: %v", r.path, err)
		}
		mapping, err := documentMapping(&root)
		if err != nil {
			return fmt.Errorf("%s: %w", r.path, err)
		}
		ensureVersion(mapping)
		syncContexts(mapping, m.Contexts)
		syncRelations(mapping, m.Relations)
	case os.IsNotExist(err):
		root = *freshDocument(m, r.schemaModeline())
	default:
		return fmt.Errorf("read %s: %w", r.path, err)
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&root); err != nil {
		_ = enc.Close()
		return fmt.Errorf("encode %s: %w", r.path, err)
	}
	if err := enc.Close(); err != nil {
		return fmt.Errorf("encode %s: %w", r.path, err)
	}
	return atomicWrite(r.path, buf.Bytes())
}

// schemaModeline chooses the editor schema hint for a freshly created
// file: relative local path when library.schema.json exists under the
// bound root, else the published raw-GitHub $id.
func (r Repository) schemaModeline() string {
	local := filepath.Join(r.root, filepath.FromSlash(localSchemaRel))
	if st, err := os.Stat(local); err == nil && !st.IsDir() {
		return "# yaml-language-server: $schema=" + localSchemaRel
	}
	return "# yaml-language-server: $schema=" + remoteSchemaURL
}

type documentDoc struct {
	Version   int           `yaml:"version"`
	Contexts  []contextDoc  `yaml:"contexts"`
	Relations []relationDoc `yaml:"relations"`
}

type contextDoc struct {
	Name           string             `yaml:"name"`
	Entities       []entityDoc        `yaml:"entities"`
	ValueObjects   []defDoc           `yaml:"value_objects"`
	Invariants     []invariantDoc     `yaml:"invariants"`
	Assertions     []assertionDoc     `yaml:"assertions"`
	Specifications []specificationDoc `yaml:"specifications"`
	Events         []eventDoc         `yaml:"events"`
}

type entityDoc struct {
	Name       string   `yaml:"name"`
	Definition string   `yaml:"definition"`
	Aliases    []string `yaml:"aliases"`
	Aggregate  bool     `yaml:"aggregate"`
}

type defDoc struct {
	Name       string   `yaml:"name"`
	Definition string   `yaml:"definition"`
	Aliases    []string `yaml:"aliases"`
}

// eventDoc intentionally omits aliases: the litmus schema forbids them
// on events (additionalProperties: false).
type eventDoc struct {
	Name       string `yaml:"name"`
	Definition string `yaml:"definition"`
}

type invariantDoc struct {
	Statement string `yaml:"statement"`
	Owner     string `yaml:"owner"`
	ID        string `yaml:"id"`
}

type assertionDoc struct {
	Statement string `yaml:"statement"`
	Owner     string `yaml:"owner"`
	ID        string `yaml:"id"`
	On        string `yaml:"on"`
}

type specificationDoc struct {
	Name       string `yaml:"name"`
	Definition string `yaml:"definition"`
}

type relationDoc struct {
	From string `yaml:"from"`
	To   string `yaml:"to"`
	Kind string `yaml:"kind"`
}

// entry is the node-surgery view of one named YAML list item
// (entity, value object, or event).
type entry struct {
	Name       string
	Definition string
	Aliases    []string
	Aggregate  bool
	// withAliases controls whether aliases may appear on the node.
	// Events never carry aliases in the authored YAML.
	withAliases bool
	// entity controls whether aggregate is legal on the node.
	entity bool
}

// invEntry is the node-surgery view of one invariant list item.
type invEntry struct {
	Statement string
	Owner     string
	ID        string
}

// assertionEntry is the node-surgery view of one assertion list item.
type assertionEntry struct {
	Statement string
	Owner     string
	ID        string
	On        string
}

// specEntry is the node-surgery view of one specification list item.
type specEntry struct {
	Name       string
	Definition string
}

// relEntry is the node-surgery view of one relation list item.
type relEntry struct {
	From string
	To   string
	Kind string
}

func toDefs(docs []defDoc, lines []int) []vocab.Definition {
	out := make([]vocab.Definition, len(docs))
	for i, d := range docs {
		out[i] = vocab.Definition{
			Name:       d.Name,
			Definition: d.Definition,
			Aliases:    d.Aliases,
			Line:       lineAt(lines, i),
		}
	}
	return out
}

func toEventDefs(docs []eventDoc, lines []int) []vocab.Definition {
	out := make([]vocab.Definition, len(docs))
	for i, d := range docs {
		out[i] = vocab.Definition{
			Name:       d.Name,
			Definition: d.Definition,
			Line:       lineAt(lines, i),
		}
	}
	return out
}

func entitiesAsEntries(entities []vocab.Entity) []entry {
	out := make([]entry, len(entities))
	for i, e := range entities {
		out[i] = entry{
			Name:        e.Name,
			Definition:  e.Definition.Definition,
			Aliases:     e.Aliases,
			Aggregate:   e.Aggregate,
			withAliases: true,
			entity:      true,
		}
	}
	return out
}

func defsAsEntries(defs []vocab.Definition) []entry {
	out := make([]entry, len(defs))
	for i, d := range defs {
		out[i] = entry{
			Name:        d.Name,
			Definition:  d.Definition,
			Aliases:     d.Aliases,
			withAliases: true,
		}
	}
	return out
}

func eventsAsEntries(defs []vocab.Definition) []entry {
	out := make([]entry, len(defs))
	for i, d := range defs {
		out[i] = entry{
			Name:       d.Name,
			Definition: d.Definition,
		}
	}
	return out
}

func invariantsAsEntries(invs []vocab.Invariant) []invEntry {
	out := make([]invEntry, len(invs))
	for i, inv := range invs {
		out[i] = invEntry{Statement: inv.Statement, Owner: inv.Owner, ID: inv.ID}
	}
	return out
}

func assertionsAsEntries(assertions []vocab.Assertion) []assertionEntry {
	out := make([]assertionEntry, len(assertions))
	for i, a := range assertions {
		out[i] = assertionEntry{Statement: a.Statement, Owner: a.Owner, ID: a.ID, On: a.On}
	}
	return out
}

func specificationsAsEntries(specs []vocab.Specification) []specEntry {
	out := make([]specEntry, len(specs))
	for i, s := range specs {
		out[i] = specEntry{Name: s.Name, Definition: s.Definition}
	}
	return out
}

func relationsAsEntries(rels []vocab.ContextRelation) []relEntry {
	out := make([]relEntry, len(rels))
	for i, r := range rels {
		out[i] = relEntry{From: r.From, To: r.To, Kind: string(r.Kind)}
	}
	return out
}

// YAML resolver tags for the node surgery below.
const (
	tagStr = "!!str"
	tagInt = "!!int"
	tagSeq = "!!seq"
	tagMap = "!!map"
)

func freshDocument(m vocab.UbiquitousLanguage, modeline string) *yaml.Node {
	mapping := &yaml.Node{Kind: yaml.MappingNode, Tag: tagMap}
	versionKey := &yaml.Node{
		Kind:        yaml.ScalarNode,
		Tag:         tagStr,
		Value:       "version",
		HeadComment: modeline,
	}
	versionVal := &yaml.Node{Kind: yaml.ScalarNode, Tag: tagInt, Value: fmt.Sprintf("%d", vocab.UbiquitousLanguageVersion)}
	mapping.Content = []*yaml.Node{versionKey, versionVal}

	// contexts is required by the schema even when empty.
	appendKV(mapping, "contexts", buildContextsSeq(m.Contexts))
	if len(m.Relations) > 0 {
		appendKV(mapping, "relations", buildRelationsSeq(relationsAsEntries(m.Relations)))
	}

	return &yaml.Node{
		Kind:    yaml.DocumentNode,
		Content: []*yaml.Node{mapping},
	}
}

func buildContextsSeq(contexts []vocab.BoundedContext) *yaml.Node {
	seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: tagSeq}
	for _, c := range contexts {
		seq.Content = append(seq.Content, buildContextNode(c))
	}
	return seq
}

func buildContextNode(c vocab.BoundedContext) *yaml.Node {
	m := &yaml.Node{Kind: yaml.MappingNode, Tag: tagMap}
	appendKV(m, keyName, stringScalar(c.Name))
	appendNamedSection(m, "entities", entitiesAsEntries(c.Entities))
	appendNamedSection(m, "value_objects", defsAsEntries(c.ValueObjects))
	appendInvariantSection(m, invariantsAsEntries(c.Invariants))
	appendAssertionSection(m, assertionsAsEntries(c.Assertions))
	appendSpecificationSection(m, specificationsAsEntries(c.Specifications))
	appendNamedSection(m, "events", eventsAsEntries(c.Events))
	return m
}

func appendNamedSection(mapping *yaml.Node, key string, entries []entry) {
	if len(entries) == 0 {
		return
	}
	seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: tagSeq}
	for _, e := range entries {
		seq.Content = append(seq.Content, buildDefNode(e))
	}
	appendKV(mapping, key, seq)
}

func appendInvariantSection(mapping *yaml.Node, entries []invEntry) {
	if len(entries) == 0 {
		return
	}
	seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: tagSeq}
	for _, e := range entries {
		seq.Content = append(seq.Content, buildInvNode(e))
	}
	appendKV(mapping, "invariants", seq)
}

func appendAssertionSection(mapping *yaml.Node, entries []assertionEntry) {
	if len(entries) == 0 {
		return
	}
	seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: tagSeq}
	for _, e := range entries {
		seq.Content = append(seq.Content, buildAssertionNode(e))
	}
	appendKV(mapping, "assertions", seq)
}

func appendSpecificationSection(mapping *yaml.Node, entries []specEntry) {
	if len(entries) == 0 {
		return
	}
	seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: tagSeq}
	for _, e := range entries {
		seq.Content = append(seq.Content, buildSpecNode(e))
	}
	appendKV(mapping, "specifications", seq)
}

func buildRelationsSeq(entries []relEntry) *yaml.Node {
	seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: tagSeq}
	for _, e := range entries {
		seq.Content = append(seq.Content, buildRelNode(e))
	}
	return seq
}

func documentMapping(root *yaml.Node) (*yaml.Node, error) {
	switch {
	case root.Kind == yaml.DocumentNode && len(root.Content) > 0:
		m := root.Content[0]
		if m.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("document root must be a mapping")
		}
		return m, nil
	case root.Kind == yaml.MappingNode:
		return root, nil
	default:
		return nil, fmt.Errorf("document root must be a mapping")
	}
}

func ensureVersion(mapping *yaml.Node) {
	val, idx := findMapEntry(mapping, "version")
	if idx >= 0 {
		val.Kind = yaml.ScalarNode
		val.Tag = tagInt
		val.Value = fmt.Sprintf("%d", vocab.UbiquitousLanguageVersion)
		return
	}
	key := stringScalar("version")
	value := &yaml.Node{Kind: yaml.ScalarNode, Tag: tagInt, Value: fmt.Sprintf("%d", vocab.UbiquitousLanguageVersion)}
	mapping.Content = append([]*yaml.Node{key, value}, mapping.Content...)
}

// syncContexts reconciles the top-level contexts sequence by context
// name: preserve surviving nodes (and their comments/order), update
// in place, append new contexts, drop removed ones. Nested sections
// inside each context are synced by their own keying rules.
func syncContexts(mapping *yaml.Node, contexts []vocab.BoundedContext) {
	wanted := make(map[string]vocab.BoundedContext, len(contexts))
	order := make([]string, 0, len(contexts))
	for _, c := range contexts {
		wanted[c.Name] = c
		order = append(order, c.Name)
	}

	seq, idx := findMapEntry(mapping, "contexts")
	if idx < 0 || seq == nil || seq.Kind != yaml.SequenceNode {
		seq = buildContextsSeq(contexts)
		// Insert after version when possible.
		insertField(mapping, "contexts", seq, docKeyOrder)
		return
	}

	seen := make(map[string]bool, len(contexts))
	kept := make([]*yaml.Node, 0, len(contexts))
	for _, item := range seq.Content {
		name := mappingName(item)
		c, ok := wanted[name]
		if !ok {
			continue
		}
		updateContextNode(item, c)
		kept = append(kept, item)
		seen[name] = true
	}
	for _, name := range order {
		if seen[name] {
			continue
		}
		kept = append(kept, buildContextNode(wanted[name]))
	}
	seq.Content = kept
}

func updateContextNode(item *yaml.Node, c vocab.BoundedContext) {
	if item.Kind != yaml.MappingNode {
		*item = *buildContextNode(c)
		return
	}
	setStringField(item, keyName, c.Name, contextKeyOrder, false)
	syncNamedSection(item, "entities", entitiesAsEntries(c.Entities))
	syncNamedSection(item, "value_objects", defsAsEntries(c.ValueObjects))
	syncInvariants(item, invariantsAsEntries(c.Invariants))
	syncAssertions(item, assertionsAsEntries(c.Assertions))
	syncSpecifications(item, specificationsAsEntries(c.Specifications))
	syncNamedSection(item, "events", eventsAsEntries(c.Events))
}

// syncNamedSection reconciles a named sequence keyed by item name.
// Empty desired content removes the section key entirely.
func syncNamedSection(mapping *yaml.Node, key string, entries []entry) {
	seq, idx := findMapEntry(mapping, key)
	if len(entries) == 0 {
		if idx >= 0 {
			mapping.Content = append(mapping.Content[:idx], mapping.Content[idx+2:]...)
		}
		return
	}

	if idx < 0 || seq == nil || seq.Kind != yaml.SequenceNode {
		seq = &yaml.Node{Kind: yaml.SequenceNode, Tag: tagSeq}
		for _, e := range entries {
			seq.Content = append(seq.Content, buildDefNode(e))
		}
		insertField(mapping, key, seq, contextKeyOrder)
		return
	}

	wanted := make(map[string]entry, len(entries))
	order := make([]string, 0, len(entries))
	for _, e := range entries {
		wanted[e.Name] = e
		order = append(order, e.Name)
	}

	seen := make(map[string]bool, len(entries))
	kept := make([]*yaml.Node, 0, len(entries))
	for _, item := range seq.Content {
		name := mappingName(item)
		e, ok := wanted[name]
		if !ok {
			continue
		}
		updateDefNode(item, e)
		kept = append(kept, item)
		seen[name] = true
	}
	for _, name := range order {
		if seen[name] {
			continue
		}
		kept = append(kept, buildDefNode(wanted[name]))
	}
	seq.Content = kept
}

// syncInvariants reconciles the invariants sequence keyed by statement.
func syncInvariants(mapping *yaml.Node, entries []invEntry) {
	const key = "invariants"
	seq, idx := findMapEntry(mapping, key)
	if len(entries) == 0 {
		if idx >= 0 {
			mapping.Content = append(mapping.Content[:idx], mapping.Content[idx+2:]...)
		}
		return
	}

	if idx < 0 || seq == nil || seq.Kind != yaml.SequenceNode {
		seq = &yaml.Node{Kind: yaml.SequenceNode, Tag: tagSeq}
		for _, e := range entries {
			seq.Content = append(seq.Content, buildInvNode(e))
		}
		insertField(mapping, key, seq, contextKeyOrder)
		return
	}

	wanted := make(map[string]invEntry, len(entries))
	order := make([]string, 0, len(entries))
	for _, e := range entries {
		wanted[e.Statement] = e
		order = append(order, e.Statement)
	}

	seen := make(map[string]bool, len(entries))
	kept := make([]*yaml.Node, 0, len(entries))
	for _, item := range seq.Content {
		stmt := mappingField(item, keyStatement)
		e, ok := wanted[stmt]
		if !ok {
			continue
		}
		updateInvNode(item, e)
		kept = append(kept, item)
		seen[stmt] = true
	}
	for _, stmt := range order {
		if seen[stmt] {
			continue
		}
		kept = append(kept, buildInvNode(wanted[stmt]))
	}
	seq.Content = kept
}

// syncAssertions reconciles the assertions sequence keyed by statement.
func syncAssertions(mapping *yaml.Node, entries []assertionEntry) {
	const key = "assertions"
	seq, idx := findMapEntry(mapping, key)
	if len(entries) == 0 {
		if idx >= 0 {
			mapping.Content = append(mapping.Content[:idx], mapping.Content[idx+2:]...)
		}
		return
	}

	if idx < 0 || seq == nil || seq.Kind != yaml.SequenceNode {
		seq = &yaml.Node{Kind: yaml.SequenceNode, Tag: tagSeq}
		for _, e := range entries {
			seq.Content = append(seq.Content, buildAssertionNode(e))
		}
		insertField(mapping, key, seq, contextKeyOrder)
		return
	}

	wanted := make(map[string]assertionEntry, len(entries))
	order := make([]string, 0, len(entries))
	for _, e := range entries {
		wanted[e.Statement] = e
		order = append(order, e.Statement)
	}

	seen := make(map[string]bool, len(entries))
	kept := make([]*yaml.Node, 0, len(entries))
	for _, item := range seq.Content {
		stmt := mappingField(item, keyStatement)
		e, ok := wanted[stmt]
		if !ok {
			continue
		}
		updateAssertionNode(item, e)
		kept = append(kept, item)
		seen[stmt] = true
	}
	for _, stmt := range order {
		if seen[stmt] {
			continue
		}
		kept = append(kept, buildAssertionNode(wanted[stmt]))
	}
	seq.Content = kept
}

// syncSpecifications reconciles the specifications sequence keyed by name.
func syncSpecifications(mapping *yaml.Node, entries []specEntry) {
	const key = "specifications"
	seq, idx := findMapEntry(mapping, key)
	if len(entries) == 0 {
		if idx >= 0 {
			mapping.Content = append(mapping.Content[:idx], mapping.Content[idx+2:]...)
		}
		return
	}

	if idx < 0 || seq == nil || seq.Kind != yaml.SequenceNode {
		seq = &yaml.Node{Kind: yaml.SequenceNode, Tag: tagSeq}
		for _, e := range entries {
			seq.Content = append(seq.Content, buildSpecNode(e))
		}
		insertField(mapping, key, seq, contextKeyOrder)
		return
	}

	wanted := make(map[string]specEntry, len(entries))
	order := make([]string, 0, len(entries))
	for _, e := range entries {
		wanted[e.Name] = e
		order = append(order, e.Name)
	}

	seen := make(map[string]bool, len(entries))
	kept := make([]*yaml.Node, 0, len(entries))
	for _, item := range seq.Content {
		name := mappingName(item)
		e, ok := wanted[name]
		if !ok {
			continue
		}
		updateSpecNode(item, e)
		kept = append(kept, item)
		seen[name] = true
	}
	for _, name := range order {
		if seen[name] {
			continue
		}
		kept = append(kept, buildSpecNode(wanted[name]))
	}
	seq.Content = kept
}

// syncRelations reconciles the top-level relations sequence keyed by
// (from, to). Empty desired content removes the key.
func syncRelations(mapping *yaml.Node, rels []vocab.ContextRelation) {
	entries := relationsAsEntries(rels)
	const key = "relations"
	seq, idx := findMapEntry(mapping, key)
	if len(entries) == 0 {
		if idx >= 0 {
			mapping.Content = append(mapping.Content[:idx], mapping.Content[idx+2:]...)
		}
		return
	}

	if idx < 0 || seq == nil || seq.Kind != yaml.SequenceNode {
		insertField(mapping, key, buildRelationsSeq(entries), docKeyOrder)
		return
	}

	wanted := make(map[string]relEntry, len(entries))
	order := make([]string, 0, len(entries))
	for _, e := range entries {
		k := relationKey(e.From, e.To)
		wanted[k] = e
		order = append(order, k)
	}

	seen := make(map[string]bool, len(entries))
	kept := make([]*yaml.Node, 0, len(entries))
	for _, item := range seq.Content {
		k := relationKey(mappingField(item, "from"), mappingField(item, "to"))
		e, ok := wanted[k]
		if !ok {
			continue
		}
		updateRelNode(item, e)
		kept = append(kept, item)
		seen[k] = true
	}
	for _, k := range order {
		if seen[k] {
			continue
		}
		kept = append(kept, buildRelNode(wanted[k]))
	}
	seq.Content = kept
}

func relationKey(from, to string) string {
	return from + "\x00" + to
}

func mappingName(item *yaml.Node) string {
	return mappingField(item, keyName)
}

func mappingField(item *yaml.Node, key string) string {
	if item == nil || item.Kind != yaml.MappingNode {
		return ""
	}
	val, idx := findMapEntry(item, key)
	if idx < 0 || val == nil {
		return ""
	}
	return val.Value
}

func updateDefNode(item *yaml.Node, e entry) {
	if item.Kind != yaml.MappingNode {
		*item = *buildDefNode(e)
		return
	}
	order := defKeyOrder
	if e.entity {
		order = entityKeyOrder
	} else if !e.withAliases {
		order = eventKeyOrder
	}
	setStringField(item, keyName, e.Name, order, false)
	setStringField(item, keyDefinition, e.Definition, order, true)
	if e.withAliases {
		setAliasesField(item, e.Aliases, order)
	} else {
		removeField(item, "aliases")
	}
	if e.entity {
		setAggregateField(item, e.Aggregate, order)
	} else {
		removeField(item, "aggregate")
	}
}

func updateInvNode(item *yaml.Node, e invEntry) {
	if item.Kind != yaml.MappingNode {
		*item = *buildInvNode(e)
		return
	}
	setStringField(item, keyStatement, e.Statement, invKeyOrder, false)
	setStringField(item, keyOwner, e.Owner, invKeyOrder, false)
	setStringField(item, keyID, e.ID, invKeyOrder, true)
}

func updateAssertionNode(item *yaml.Node, e assertionEntry) {
	if item.Kind != yaml.MappingNode {
		*item = *buildAssertionNode(e)
		return
	}
	setStringField(item, keyStatement, e.Statement, assertionKeyOrder, false)
	setStringField(item, keyOwner, e.Owner, assertionKeyOrder, false)
	setStringField(item, keyID, e.ID, assertionKeyOrder, false)
	setStringField(item, keyOn, e.On, assertionKeyOrder, false)
}

func updateSpecNode(item *yaml.Node, e specEntry) {
	if item.Kind != yaml.MappingNode {
		*item = *buildSpecNode(e)
		return
	}
	setStringField(item, keyName, e.Name, specKeyOrder, false)
	setStringField(item, keyDefinition, e.Definition, specKeyOrder, false)
}

func updateRelNode(item *yaml.Node, e relEntry) {
	if item.Kind != yaml.MappingNode {
		*item = *buildRelNode(e)
		return
	}
	setStringField(item, "from", e.From, relKeyOrder, false)
	setStringField(item, "to", e.To, relKeyOrder, false)
	setStringField(item, "kind", e.Kind, relKeyOrder, false)
}

// The recurring mapping keys of the contexts document.
const (
	keyName       = "name"
	keyDefinition = "definition"
	keyStatement  = "statement"
	keyOwner      = "owner"
	keyID         = "id"
	keyOn         = "on"
)

var (
	docKeyOrder       = []string{"version", "contexts", "relations"}
	contextKeyOrder   = []string{keyName, "entities", "value_objects", "invariants", "assertions", "specifications", "events"}
	entityKeyOrder    = []string{keyName, keyDefinition, "aliases", "aggregate"}
	defKeyOrder       = []string{keyName, keyDefinition, "aliases"}
	eventKeyOrder     = []string{keyName, keyDefinition}
	invKeyOrder       = []string{keyStatement, keyOwner, keyID}
	assertionKeyOrder = []string{keyStatement, keyOwner, keyID, keyOn}
	specKeyOrder      = []string{keyName, keyDefinition}
	relKeyOrder       = []string{"from", "to", "kind"}
)

func buildDefNode(e entry) *yaml.Node {
	m := &yaml.Node{Kind: yaml.MappingNode, Tag: tagMap}
	appendKV(m, keyName, stringScalar(e.Name))
	if e.Definition != "" {
		appendKV(m, keyDefinition, stringScalar(e.Definition))
	}
	if e.withAliases && len(e.Aliases) > 0 {
		appendKV(m, "aliases", stringSequence(e.Aliases))
	}
	if e.entity && e.Aggregate {
		appendKV(m, "aggregate", boolScalar(true))
	}
	return m
}

func buildInvNode(e invEntry) *yaml.Node {
	m := &yaml.Node{Kind: yaml.MappingNode, Tag: tagMap}
	appendKV(m, keyStatement, stringScalar(e.Statement))
	appendKV(m, keyOwner, stringScalar(e.Owner))
	if e.ID != "" {
		appendKV(m, keyID, stringScalar(e.ID))
	}
	return m
}

func buildAssertionNode(e assertionEntry) *yaml.Node {
	m := &yaml.Node{Kind: yaml.MappingNode, Tag: tagMap}
	appendKV(m, keyStatement, stringScalar(e.Statement))
	appendKV(m, keyOwner, stringScalar(e.Owner))
	appendKV(m, keyID, stringScalar(e.ID))
	appendKV(m, keyOn, stringScalar(e.On))
	return m
}

func buildSpecNode(e specEntry) *yaml.Node {
	m := &yaml.Node{Kind: yaml.MappingNode, Tag: tagMap}
	appendKV(m, keyName, stringScalar(e.Name))
	if e.Definition != "" {
		appendKV(m, keyDefinition, stringScalar(e.Definition))
	}
	return m
}

func buildRelNode(e relEntry) *yaml.Node {
	m := &yaml.Node{Kind: yaml.MappingNode, Tag: tagMap}
	appendKV(m, "from", stringScalar(e.From))
	appendKV(m, "to", stringScalar(e.To))
	appendKV(m, "kind", stringScalar(e.Kind))
	return m
}

func setStringField(mapping *yaml.Node, key, value string, order []string, removeEmpty bool) {
	if value == "" && removeEmpty {
		removeField(mapping, key)
		return
	}
	setField(mapping, key, stringScalar(value), order)
}

func setAliasesField(mapping *yaml.Node, aliases []string, order []string) {
	if len(aliases) == 0 {
		removeField(mapping, "aliases")
		return
	}
	setField(mapping, "aliases", stringSequence(aliases), order)
}

func setAggregateField(mapping *yaml.Node, aggregate bool, order []string) {
	if !aggregate {
		removeField(mapping, "aggregate")
		return
	}
	setField(mapping, "aggregate", boolScalar(true), order)
}

func setField(mapping *yaml.Node, key string, value *yaml.Node, order []string) {
	existing, idx := findMapEntry(mapping, key)
	if idx >= 0 {
		// Preserve comments hanging off the previous value node by
		// copying scalar/sequence content into the existing node when
		// kinds match; otherwise replace the value slot.
		if existing != nil && existing.Kind == value.Kind {
			existing.Tag = value.Tag
			existing.Value = value.Value
			existing.Content = value.Content
			return
		}
		mapping.Content[idx+1] = value
		return
	}
	insertField(mapping, key, value, order)
}

func insertField(mapping *yaml.Node, key string, value *yaml.Node, order []string) {
	rank := indexOf(order, key)
	insertAt := len(mapping.Content)
	if rank >= 0 {
		insertAt = 0
		for i := 0; i+1 < len(mapping.Content); i += 2 {
			r := indexOf(order, mapping.Content[i].Value)
			if r >= 0 && r < rank {
				insertAt = i + 2
			}
		}
	}
	keyNode := stringScalar(key)
	tail := append([]*yaml.Node{keyNode, value}, mapping.Content[insertAt:]...)
	mapping.Content = append(mapping.Content[:insertAt], tail...)
}

func removeField(mapping *yaml.Node, key string) {
	_, idx := findMapEntry(mapping, key)
	if idx < 0 {
		return
	}
	mapping.Content = append(mapping.Content[:idx], mapping.Content[idx+2:]...)
}

func findMapEntry(mapping *yaml.Node, key string) (valueNode *yaml.Node, index int) {
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
	mapping.Content = append(mapping.Content,
		stringScalar(key),
		value,
	)
}

func stringScalar(v string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: tagStr, Value: v}
}

func boolScalar(v bool) *yaml.Node {
	if v {
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "true"}
	}
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "false"}
}

func stringSequence(values []string) *yaml.Node {
	seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: tagSeq}
	for _, v := range values {
		seq.Content = append(seq.Content, stringScalar(v))
	}
	return seq
}

// atomicWrite writes data to path via a same-directory temp file and
// rename, leaving no temp residue on success or failure. Mode 0o600
// matches the baseline JSON store.
func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".ubiquitous-language-*.yaml")
	if err != nil {
		return fmt.Errorf("create temp for %s: %w", path, err)
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp for %s: %w", path, err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp for %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp for %s: %w", path, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename temp onto %s: %w", path, err)
	}
	cleanup = false
	return nil
}
