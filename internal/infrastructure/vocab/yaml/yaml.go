// Package yamlvocab loads and stores the project's recorded
// Ubiquitous Language vocabulary from ubiquitous-language.yaml.
// RecordedLanguage is strict (unknown keys are errors). Record
// preserves human authoring of untouched entries — comments and
// ordering — and writes atomically.
package yamlvocab

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// schemaModeline is the editor schema hint written on freshly created
// ubiquitous-language.yaml files. The raw-GitHub URL pattern matches
// the rules.schema.json modeline in getting-started.md.
const schemaModeline = "# yaml-language-server: $schema=https://raw.githubusercontent.com/wixregiga/arclint/main/docs/ubiquitous-language.schema.json"

// Repository implements the domain-owned vocab.Repository port over
// one ubiquitous-language.yaml beside the resolved ruleset root.
type Repository struct {
	path string
}

// NewRepository binds the repository to filepath.Join(root, UbiquitousLanguageFileName).
func NewRepository(root string) (Repository, error) {
	if root == "" {
		return Repository{}, fmt.Errorf("ubiquitous-language root: empty path")
	}
	abs, err := filepath.Abs(filepath.Join(root, vocab.UbiquitousLanguageFileName))
	if err != nil {
		return Repository{}, fmt.Errorf("ubiquitous-language path: %w", err)
	}
	return Repository{path: abs}, nil
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
// rather than a repository file — such as the rule-test harness
// feeding ctx.domain() — parse through here.
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

	entities := make([]vocab.Entity, len(doc.Entities))
	for i, e := range doc.Entities {
		entities[i] = vocab.Entity{
			Definition: vocab.Definition{
				Name:       e.Name,
				Definition: e.Definition,
				Aliases:    e.Aliases,
			},
			Aggregate: e.Aggregate,
		}
	}
	lang, err := vocab.NewUbiquitousLanguage(
		entities,
		toDefs(doc.ValueObjects),
		toDefs(doc.BusinessRules),
		toDefs(doc.Events),
	)
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
		syncSection(mapping, "entities", entitiesAsEntries(m.Entities), true)
		syncSection(mapping, "value_objects", defsAsEntries(m.ValueObjects), false)
		syncSection(mapping, "business_rules", defsAsEntries(m.BusinessRules), false)
		syncSection(mapping, "events", defsAsEntries(m.Events), false)
	case os.IsNotExist(err):
		root = *freshDocument(m)
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

type documentDoc struct {
	Version       int         `yaml:"version"`
	Entities      []entityDoc `yaml:"entities"`
	ValueObjects  []defDoc    `yaml:"value_objects"`
	BusinessRules []defDoc    `yaml:"business_rules"`
	Events        []defDoc    `yaml:"events"`
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

// entry is the node-surgery view of one YAML list item.
type entry struct {
	Name       string
	Definition string
	Aliases    []string
	Aggregate  bool
}

func toDefs(docs []defDoc) []vocab.Definition {
	out := make([]vocab.Definition, len(docs))
	for i, d := range docs {
		out[i] = vocab.Definition{
			Name:       d.Name,
			Definition: d.Definition,
			Aliases:    d.Aliases,
		}
	}
	return out
}

func entitiesAsEntries(entities []vocab.Entity) []entry {
	out := make([]entry, len(entities))
	for i, e := range entities {
		out[i] = entry{
			Name:       e.Name,
			Definition: e.Definition.Definition,
			Aliases:    e.Aliases,
			Aggregate:  e.Aggregate,
		}
	}
	return out
}

func defsAsEntries(defs []vocab.Definition) []entry {
	out := make([]entry, len(defs))
	for i, d := range defs {
		out[i] = entry{
			Name:       d.Name,
			Definition: d.Definition,
			Aliases:    d.Aliases,
		}
	}
	return out
}

// YAML resolver tags for the node surgery below.
const (
	tagStr = "!!str"
	tagInt = "!!int"
	tagSeq = "!!seq"
)

func freshDocument(m vocab.UbiquitousLanguage) *yaml.Node {
	mapping := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	versionKey := &yaml.Node{
		Kind:        yaml.ScalarNode,
		Tag:         tagStr,
		Value:       "version",
		HeadComment: schemaModeline,
	}
	versionVal := &yaml.Node{Kind: yaml.ScalarNode, Tag: tagInt, Value: fmt.Sprintf("%d", vocab.UbiquitousLanguageVersion)}
	mapping.Content = []*yaml.Node{versionKey, versionVal}

	appendSection := func(key string, entries []entry, entity bool) {
		if len(entries) == 0 {
			return
		}
		seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: tagSeq}
		for _, e := range entries {
			seq.Content = append(seq.Content, buildDefNode(e, entity))
		}
		mapping.Content = append(mapping.Content,
			stringScalar(key),
			seq,
		)
	}
	appendSection("entities", entitiesAsEntries(m.Entities), true)
	appendSection("value_objects", defsAsEntries(m.ValueObjects), false)
	appendSection("business_rules", defsAsEntries(m.BusinessRules), false)
	appendSection("events", defsAsEntries(m.Events), false)

	return &yaml.Node{
		Kind:    yaml.DocumentNode,
		Content: []*yaml.Node{mapping},
	}
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

func syncSection(mapping *yaml.Node, key string, entries []entry, entity bool) {
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
			seq.Content = append(seq.Content, buildDefNode(e, entity))
		}
		mapping.Content = append(mapping.Content,
			stringScalar(key),
			seq,
		)
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
		updateDefNode(item, e, entity)
		kept = append(kept, item)
		seen[name] = true
	}
	for _, name := range order {
		if seen[name] {
			continue
		}
		kept = append(kept, buildDefNode(wanted[name], entity))
	}
	seq.Content = kept
}

func mappingName(item *yaml.Node) string {
	if item == nil || item.Kind != yaml.MappingNode {
		return ""
	}
	val, idx := findMapEntry(item, "name")
	if idx < 0 || val == nil {
		return ""
	}
	return val.Value
}

func updateDefNode(item *yaml.Node, e entry, entity bool) {
	if item.Kind != yaml.MappingNode {
		*item = *buildDefNode(e, entity)
		return
	}
	order := defKeyOrder
	if entity {
		order = entityKeyOrder
	}
	setStringField(item, "name", e.Name, order, false)
	setStringField(item, "definition", e.Definition, order, true)
	setAliasesField(item, e.Aliases, order)
	if entity {
		setAggregateField(item, e.Aggregate, order)
	} else {
		removeField(item, "aggregate")
	}
}

var (
	entityKeyOrder = []string{"name", "definition", "aliases", "aggregate"}
	defKeyOrder    = []string{"name", "definition", "aliases"}
)

func buildDefNode(e entry, entity bool) *yaml.Node {
	m := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	appendKV(m, "name", stringScalar(e.Name))
	if e.Definition != "" {
		appendKV(m, "definition", stringScalar(e.Definition))
	}
	if len(e.Aliases) > 0 {
		appendKV(m, "aliases", stringSequence(e.Aliases))
	}
	if entity && e.Aggregate {
		appendKV(m, "aggregate", boolScalar(true))
	}
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
