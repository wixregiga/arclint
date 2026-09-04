// Package yamlrule loads complete Rule aggregates from rules.yaml. The
// accepted grammar is one document shape for both file kinds: a
// repository ruleset carries runtime, scan, extends, modules, and
// rules; a Pattern distribution file carries the pattern header,
// modules, and rules. Every Rule is keyed by its Rule ID and carries
// exactly one assertion key, which decides its Type; an entry with no
// assertion key is an Override of a Rule an extended Pattern
// distributes. Compact spellings are sugar the loader expands: the
// engine only ever sees complete Modules, Rules, and Patterns. A
// representation that cannot become a valid value is an error, never a
// partial value.
package yamlrule

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// PatternSource supplies the Patterns a repository may extend, as
// validated domain values.
type PatternSource interface {
	Patterns() ([]rule.Pattern, error)
}

// Repository implements the domain-owned rule.Repository port over one
// ruleset file. The recorded Ubiquitous Language is an input to rule
// configuration: expanded structure Rules resolve their globs against
// it at load time, so the engine only ever sees ordinary Rules. The
// Pattern sources supply what extends may name. The configuration is
// loaded once and memoized: every use case in one invocation sees the
// same Rules.
type Repository struct {
	path      string
	knowledge vocab.Repository
	patterns  []PatternSource

	once       sync.Once
	configured rule.Configured
	err        error
}

// NewRepository binds the repository to a ruleset file path, the
// recorded-vocabulary port expanded Rules resolve against, and the
// Pattern sources extends resolves through. A nil knowledge port means
// no recorded vocabulary: expanded Rules then resolve empty, exactly as
// with an absent ubiquitous-language.yaml. No Pattern source means a
// ruleset that extends anything fails to load.
func NewRepository(path string, knowledge vocab.Repository, patterns ...PatternSource) (*Repository, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("ruleset path: %w", err)
	}
	for _, p := range patterns {
		if p == nil {
			return nil, fmt.Errorf("ruleset %s: nil pattern source", abs)
		}
	}
	return &Repository{path: abs, knowledge: knowledge, patterns: patterns}, nil
}

// Path returns the absolute ruleset file path.
func (r *Repository) Path() string { return r.path }

// Root returns the directory containing the ruleset file: the
// repository root every repo-relative path is resolved against.
func (r *Repository) Root() string { return filepath.Dir(r.path) }

// ConfiguredRules loads, validates, and translates the ruleset into
// complete Rule aggregates: its own Rules plus those of every extended
// Pattern with the repository's Overrides applied, expanded Rules
// resolved against the recorded Ubiquitous Language.
func (r *Repository) ConfiguredRules() (rule.Configured, error) {
	r.once.Do(func() { r.configured, r.err = r.load() })
	return r.configured, r.err
}

func (r *Repository) load() (rule.Configured, error) {
	data, err := os.ReadFile(r.path)
	if err != nil {
		return rule.Configured{}, fmt.Errorf("read ruleset: %w", err)
	}
	lang := vocab.UbiquitousLanguage{}
	if r.knowledge != nil {
		recorded, found, err := r.knowledge.RecordedLanguage()
		if err != nil {
			return rule.Configured{}, fmt.Errorf("load domain model: %w", err)
		}
		if found {
			lang = recorded
		}
	}
	parsed, err := parse(data, r.path)
	if err != nil {
		return rule.Configured{}, err
	}
	if parsed.pattern != nil {
		return rule.Configured{}, fmt.Errorf("%s: a pattern distribution file is not a repository ruleset", r.path)
	}
	var available []rule.Pattern
	if len(parsed.extends) > 0 {
		for _, source := range r.patterns {
			ps, err := source.Patterns()
			if err != nil {
				return rule.Configured{}, fmt.Errorf("%s: load patterns: %w", r.path, err)
			}
			available = append(available, ps...)
		}
	}
	return parsed.repository(lang, available)
}

// Document is one parsed and translated ruleset file: exactly one of
// Configured (a repository ruleset) or Pattern (a distribution file)
// is meaningful.
type Document struct {
	Configured rule.Configured
	// Pattern is non-nil for Pattern distribution files.
	Pattern *rule.Pattern
}

// Load parses one ruleset document strictly and translates it. A
// repository ruleset resolves extends against the available Patterns
// and expanded structure Rules against the given recorded language; a
// Pattern distribution file becomes a Pattern without Extension
// sources (LoadPattern attaches those).
func Load(data []byte, source string, lang vocab.UbiquitousLanguage, available []rule.Pattern) (Document, error) {
	parsed, err := parse(data, source)
	if err != nil {
		return Document{}, err
	}
	if parsed.pattern != nil {
		p, err := parsed.distribution(nil)
		if err != nil {
			return Document{}, err
		}
		return Document{Pattern: &p}, nil
	}
	cfg, err := parsed.repository(lang, available)
	if err != nil {
		return Document{}, err
	}
	return Document{Configured: cfg}, nil
}

// LoadPattern parses one Pattern distribution file and attaches the
// Extension sources it ships with.
func LoadPattern(data []byte, source string, extensions []rule.PatternExtension) (rule.Pattern, error) {
	parsed, err := parse(data, source)
	if err != nil {
		return rule.Pattern{}, err
	}
	if parsed.pattern == nil {
		return rule.Pattern{}, fmt.Errorf("%s: missing pattern header (namespace, name, version)", source)
	}
	return parsed.distribution(extensions)
}

// Top-level and nested keys of the grammar.
const (
	keyPattern       = "pattern"
	keyRuntime       = "runtime"
	keyScan          = "scan"
	keyExtends       = "extends"
	keyModules       = "modules"
	keyRules         = "rules"
	keyNamespace     = "namespace"
	keyName          = "name"
	keyVersion       = "version"
	keyCoverage      = "coverage"
	keyDocumentation = "documentation"
	keyUnknown       = "unknown_imports"
	keyExclude       = "exclude"
	keyTestdata      = "include_testdata"
	keyBind          = "bind"
	keyPaths         = "paths"
	keyDescription   = "description"
	keySeverity      = "severity"
	keyOn            = "on"
	keyFiles         = "files"
	keyWith          = "with"
	keyDisable       = "disable"
	keySuppress      = "suppress"
	keyReason        = "reason"
	keyInternal      = "internal"
	keyExternal      = "external"
	keyStdlib        = "stdlib"
	keyRequire       = "require"
	keyForbid        = "forbid"
	keyEach          = "each"
	keyCase          = "case"
	keyClosed        = "closed"
)

// document is the strictly parsed file before domain translation.
type document struct {
	source  string
	pattern *patternHeader
	runtime []rule.Language
	scan    rule.Scan
	extends []extension
	modules []moduleEntry
	rules   []ruleEntry
}

type patternHeader struct {
	namespace     string
	name          string
	version       string
	coverage      []rule.Language
	documentation string
}

type extension struct {
	where    string
	ref      rule.PatternReference
	bindings []rule.Binding
}

// moduleEntry is one modules entry; paths is nil when the entry
// carries none (legal only in a Pattern file).
type moduleEntry struct {
	where       string
	name        rule.ModuleName
	description string
	paths       []rule.Glob
	hasPaths    bool
}

// ruleEntry is one rules entry: a Rule (assertion set) or an Override
// (assertion empty).
type ruleEntry struct {
	where       string
	id          string
	description string
	severity    string
	on          []rule.ModuleName
	onPresent   bool
	files       []rule.Glob
	assertion   string
	assertNode  *yaml.Node
	with        map[string]any
	withPresent bool
	disable     *string
	exclude     *exclusionEntry
	suppress    *suppressionEntry
}

type exclusionEntry struct {
	paths   []rule.Glob
	modules []rule.ModuleName
	reason  string
}

type suppressionEntry struct {
	paths  []rule.Glob
	reason string
}

// parse reads the document strictly: unknown keys anywhere, wrong
// value shapes, and keys the file kind does not accept are errors.
func parse(data []byte, source string) (*document, error) {
	var root yaml.Node
	if err := yaml.NewDecoder(bytes.NewReader(data)).Decode(&root); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("%s: empty document", source)
		}
		return nil, fmt.Errorf("%s: %v", source, err)
	}
	body := &root
	if body.Kind == yaml.DocumentNode {
		if len(body.Content) == 0 {
			return nil, fmt.Errorf("%s: empty document", source)
		}
		body = body.Content[0]
	}
	top, err := asMapping(body, "document")
	if err != nil {
		return nil, fmt.Errorf("%s: %v", source, err)
	}
	doc := &document{source: source}
	fail := func(format string, args ...any) (*document, error) {
		return nil, fmt.Errorf("%s: %s", source, fmt.Sprintf(format, args...))
	}
	if err := top.allow(keyPattern, keyRuntime, keyScan, keyExtends, keyModules, keyRules); err != nil {
		return fail("%v", err)
	}
	if n := top.get(keyPattern); n != nil {
		header, err := parsePatternHeader(n)
		if err != nil {
			return fail("%v", err)
		}
		doc.pattern = &header
		for _, k := range []string{keyRuntime, keyScan, keyExtends} {
			if top.get(k) != nil {
				return fail("%s: a pattern distribution file does not accept %s; that is repository policy", k, k)
			}
		}
	}
	if n := top.get(keyRuntime); n != nil {
		targets, err := stringList(n, keyRuntime)
		if err != nil {
			return fail("%v", err)
		}
		if len(targets) == 0 {
			return fail("runtime: names no language; expected one or more of %s", strings.Join(runtimeTargets(), ", "))
		}
		if doc.runtime, err = translateTargets(keyRuntime, targets); err != nil {
			return fail("%v", err)
		}
	}
	if n := top.get(keyScan); n != nil {
		scan, err := parseScan(n)
		if err != nil {
			return fail("%v", err)
		}
		doc.scan = scan
	} else if doc.scan.UnknownImports, err = rule.ParseUnknownImportPolicy(""); err != nil {
		return fail("%v", err)
	}
	if n := top.get(keyExtends); n != nil {
		exts, err := parseExtends(n)
		if err != nil {
			return fail("%v", err)
		}
		doc.extends = exts
	}
	if n := top.get(keyModules); n != nil {
		mods, err := parseModules(n, doc.pattern != nil)
		if err != nil {
			return fail("%v", err)
		}
		doc.modules = mods
	}
	if n := top.get(keyRules); n != nil {
		rules, err := parseRules(n)
		if err != nil {
			return fail("%v", err)
		}
		doc.rules = rules
	}
	return doc, nil
}

func parsePatternHeader(n *yaml.Node) (patternHeader, error) {
	m, err := asMapping(n, keyPattern)
	if err != nil {
		return patternHeader{}, err
	}
	if err := m.allow(keyNamespace, keyName, keyVersion, keyCoverage, keyDocumentation); err != nil {
		return patternHeader{}, err
	}
	var h patternHeader
	for _, k := range []string{keyNamespace, keyName, keyVersion} {
		v := m.get(k)
		if v == nil {
			return patternHeader{}, fmt.Errorf("pattern: %s is required (namespace, name, and version identify a pattern)", k)
		}
		s, err := scalarString(v, keyPattern+"."+k)
		if err != nil {
			return patternHeader{}, err
		}
		switch k {
		case keyNamespace:
			h.namespace = s
		case keyName:
			h.name = s
		case keyVersion:
			h.version = s
		}
	}
	if v := m.get(keyCoverage); v != nil {
		names, err := stringList(v, "pattern.coverage")
		if err != nil {
			return patternHeader{}, err
		}
		if h.coverage, err = translateTargets("pattern.coverage", names); err != nil {
			return patternHeader{}, err
		}
	}
	if v := m.get(keyDocumentation); v != nil {
		s, err := scalarString(v, "pattern.documentation")
		if err != nil {
			return patternHeader{}, err
		}
		h.documentation = s
	}
	return h, nil
}

func parseScan(n *yaml.Node) (rule.Scan, error) {
	m, err := asMapping(n, keyScan)
	if err != nil {
		return rule.Scan{}, err
	}
	if err := m.allow(keyUnknown, keyExclude, keyTestdata); err != nil {
		return rule.Scan{}, err
	}
	var out rule.Scan
	policy := ""
	if v := m.get(keyUnknown); v != nil {
		if policy, err = scalarString(v, "scan.unknown_imports"); err != nil {
			return rule.Scan{}, err
		}
	}
	if out.UnknownImports, err = rule.ParseUnknownImportPolicy(policy); err != nil {
		return rule.Scan{}, fmt.Errorf("scan: %v", err)
	}
	if v := m.get(keyExclude); v != nil {
		patterns, err := stringList(v, "scan.exclude")
		if err != nil {
			return rule.Scan{}, err
		}
		if out.Exclude, err = rule.NewGlobs(patterns); err != nil {
			return rule.Scan{}, fmt.Errorf("scan.exclude: %v", err)
		}
	}
	if v := m.get(keyTestdata); v != nil {
		if out.IncludeTestdata, err = boolValue(v, "scan.include_testdata"); err != nil {
			return rule.Scan{}, err
		}
	}
	return out, nil
}

func parseExtends(n *yaml.Node) ([]extension, error) {
	if n.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("extends: expected a list of {pattern, bind} entries")
	}
	var out []extension
	seen := map[string]bool{}
	for i, item := range n.Content {
		where := fmt.Sprintf("extends[%d]", i)
		m, err := asMapping(item, where)
		if err != nil {
			return nil, err
		}
		if err := m.allow(keyPattern, keyBind); err != nil {
			return nil, fmt.Errorf("%s: %v", where, err)
		}
		pv := m.get(keyPattern)
		if pv == nil {
			return nil, fmt.Errorf("%s: pattern is required (namespace/name@version)", where)
		}
		spelled, err := scalarString(pv, where+".pattern")
		if err != nil {
			return nil, err
		}
		ref, err := rule.ParsePatternReference(spelled)
		if err != nil {
			return nil, fmt.Errorf("%s: %v", where, err)
		}
		if seen[ref.String()] {
			return nil, fmt.Errorf("%s: pattern %s is extended twice", where, ref)
		}
		seen[ref.String()] = true
		ext := extension{where: where, ref: ref}
		if bv := m.get(keyBind); bv != nil {
			if ext.bindings, err = parseBindings(bv, where+".bind"); err != nil {
				return nil, err
			}
		}
		out = append(out, ext)
	}
	return out, nil
}

// parseBindings reads one extends entry's bind map: Pattern Module
// name to the local paths that stand in for it.
func parseBindings(n *yaml.Node, where string) ([]rule.Binding, error) {
	bm, err := asMapping(n, where)
	if err != nil {
		return nil, err
	}
	var out []rule.Binding
	for _, key := range bm.keys() {
		name, err := rule.NewModuleName(key)
		if err != nil {
			return nil, fmt.Errorf("%s.%s: %v", where, key, err)
		}
		patterns, err := stringOrList(bm.get(key), where+"."+key)
		if err != nil {
			return nil, err
		}
		globs, err := rule.NewGlobs(patterns)
		if err != nil {
			return nil, fmt.Errorf("%s.%s: %v", where, key, err)
		}
		b, err := rule.NewBinding(name, globs)
		if err != nil {
			return nil, fmt.Errorf("%s: %v", where, err)
		}
		out = append(out, b)
	}
	return out, nil
}

// parseModules expands the module sugar. In a repository ruleset a
// string or a list is the Module's paths; in a Pattern file a string
// is its description. The object form spells both.
func parseModules(n *yaml.Node, inPattern bool) ([]moduleEntry, error) {
	m, err := asMapping(n, keyModules)
	if err != nil {
		return nil, err
	}
	var out []moduleEntry
	for _, key := range m.keys() {
		where := "modules." + key
		name, err := rule.NewModuleName(key)
		if err != nil {
			return nil, fmt.Errorf("%s: %v", where, err)
		}
		entry := moduleEntry{where: where, name: name}
		v := m.get(key)
		switch v.Kind {
		case yaml.ScalarNode:
			s, err := scalarString(v, where)
			if err != nil {
				return nil, err
			}
			if inPattern {
				entry.description = s
			} else {
				g, err := rule.NewGlob(s)
				if err != nil {
					return nil, fmt.Errorf("%s: %v", where, err)
				}
				entry.paths = []rule.Glob{g}
				entry.hasPaths = true
			}
		case yaml.SequenceNode:
			if inPattern {
				return nil, fmt.Errorf("%s: a pattern lists a module by its description; the adopting repository binds its paths", where)
			}
			patterns, err := stringList(v, where)
			if err != nil {
				return nil, err
			}
			if entry.paths, err = rule.NewGlobs(patterns); err != nil {
				return nil, fmt.Errorf("%s: %v", where, err)
			}
			entry.hasPaths = true
		case yaml.MappingNode:
			om, err := asMapping(v, where)
			if err != nil {
				return nil, err
			}
			if err := om.allow(keyPaths, keyDescription); err != nil {
				return nil, fmt.Errorf("%s: %v", where, err)
			}
			if dv := om.get(keyDescription); dv != nil {
				if entry.description, err = scalarString(dv, where+".description"); err != nil {
					return nil, err
				}
			}
			if pv := om.get(keyPaths); pv != nil {
				patterns, err := stringOrList(pv, where+".paths")
				if err != nil {
					return nil, err
				}
				if entry.paths, err = rule.NewGlobs(patterns); err != nil {
					return nil, fmt.Errorf("%s.paths: %v", where, err)
				}
				entry.hasPaths = true
			}
			if inPattern && strings.TrimSpace(entry.description) == "" {
				return nil, fmt.Errorf("%s: a pattern module requires a description", where)
			}
			if !inPattern && !entry.hasPaths {
				return nil, fmt.Errorf("%s: a repository module requires paths; a module without paths is legal only in a pattern file", where)
			}
		default:
			return nil, fmt.Errorf("%s: expected a glob, a list of globs, or an object with paths and description", where)
		}
		if entry.hasPaths && len(entry.paths) == 0 {
			return nil, fmt.Errorf("%s: paths is empty", where)
		}
		out = append(out, entry)
	}
	return out, nil
}

// ruleKeys are the keys a rules entry may carry beside its assertion.
var ruleKeys = []string{keyDescription, keySeverity, keyOn, keyFiles, keyWith, keyDisable, keyExclude, keySuppress}

func parseRules(n *yaml.Node) ([]ruleEntry, error) {
	m, err := asMapping(n, keyRules)
	if err != nil {
		return nil, err
	}
	var out []ruleEntry
	for _, id := range m.keys() {
		where := "rules." + id
		entry, err := parseRule(id, where, m.get(id))
		if err != nil {
			return nil, err
		}
		out = append(out, entry)
	}
	return out, nil
}

func parseRule(id, where string, n *yaml.Node) (ruleEntry, error) {
	m, err := asMapping(n, where)
	if err != nil {
		return ruleEntry{}, err
	}
	if err := m.allow(append(append([]string(nil), ruleKeys...), rule.AssertionKeys()...)...); err != nil {
		return ruleEntry{}, fmt.Errorf("%s: %v", where, err)
	}
	entry := ruleEntry{where: where, id: id}
	var assertions []string
	for _, key := range rule.AssertionKeys() {
		if m.get(key) != nil {
			assertions = append(assertions, key)
		}
	}
	switch len(assertions) {
	case 0:
	case 1:
		entry.assertion = assertions[0]
		entry.assertNode = m.get(assertions[0])
	default:
		return ruleEntry{}, fmt.Errorf("%s: carries %d assertions (%s); a Rule carries exactly one, so give each its own Rule ID",
			where, len(assertions), strings.Join(assertions, ", "))
	}
	if v := m.get(keyDescription); v != nil {
		if entry.description, err = scalarString(v, where+".description"); err != nil {
			return ruleEntry{}, err
		}
	}
	if v := m.get(keySeverity); v != nil {
		if entry.severity, err = scalarString(v, where+".severity"); err != nil {
			return ruleEntry{}, err
		}
	}
	if v := m.get(keyOn); v != nil {
		names, err := stringOrList(v, where+".on")
		if err != nil {
			return ruleEntry{}, err
		}
		for _, name := range names {
			mn, err := rule.NewModuleName(name)
			if err != nil {
				return ruleEntry{}, fmt.Errorf("%s.on: %v", where, err)
			}
			entry.on = append(entry.on, mn)
		}
		entry.onPresent = true
	}
	if v := m.get(keyFiles); v != nil {
		patterns, err := stringOrList(v, where+".files")
		if err != nil {
			return ruleEntry{}, err
		}
		if entry.files, err = rule.NewGlobs(patterns); err != nil {
			return ruleEntry{}, fmt.Errorf("%s.files: %v", where, err)
		}
	}
	if v := m.get(keyWith); v != nil {
		if v.Kind != yaml.MappingNode {
			return ruleEntry{}, fmt.Errorf("%s.with: expected an object of extension parameters", where)
		}
		var with map[string]any
		if err := v.Decode(&with); err != nil {
			return ruleEntry{}, fmt.Errorf("%s.with: %v", where, err)
		}
		entry.with = with
		entry.withPresent = true
	}
	if v := m.get(keyDisable); v != nil {
		reason, err := scalarString(v, where+".disable")
		if err != nil {
			return ruleEntry{}, err
		}
		if strings.TrimSpace(reason) == "" {
			return ruleEntry{}, fmt.Errorf("%s.disable: a reason is required so the decision stays inspectable", where)
		}
		entry.disable = &reason
	}
	if v := m.get(keyExclude); v != nil {
		ex, err := parseExclusion(v, where+".exclude")
		if err != nil {
			return ruleEntry{}, err
		}
		entry.exclude = &ex
	}
	if v := m.get(keySuppress); v != nil {
		su, err := parseSuppression(v, where+".suppress")
		if err != nil {
			return ruleEntry{}, err
		}
		entry.suppress = &su
	}
	return entry, nil
}

func parseExclusion(n *yaml.Node, where string) (exclusionEntry, error) {
	m, err := asMapping(n, where)
	if err != nil {
		return exclusionEntry{}, err
	}
	if err := m.allow(keyPaths, keyModules, keyReason); err != nil {
		return exclusionEntry{}, fmt.Errorf("%s: %v", where, err)
	}
	var out exclusionEntry
	if v := m.get(keyPaths); v != nil {
		patterns, err := stringOrList(v, where+".paths")
		if err != nil {
			return exclusionEntry{}, err
		}
		if out.paths, err = rule.NewGlobs(patterns); err != nil {
			return exclusionEntry{}, fmt.Errorf("%s.paths: %v", where, err)
		}
	}
	if v := m.get(keyModules); v != nil {
		names, err := stringOrList(v, where+".modules")
		if err != nil {
			return exclusionEntry{}, err
		}
		for _, name := range names {
			mn, err := rule.NewModuleName(name)
			if err != nil {
				return exclusionEntry{}, fmt.Errorf("%s.modules: %v", where, err)
			}
			out.modules = append(out.modules, mn)
		}
	}
	if out.reason, err = requiredReason(m, where); err != nil {
		return exclusionEntry{}, err
	}
	if len(out.paths)+len(out.modules) == 0 {
		return exclusionEntry{}, fmt.Errorf("%s: names no paths and no modules", where)
	}
	return out, nil
}

func parseSuppression(n *yaml.Node, where string) (suppressionEntry, error) {
	m, err := asMapping(n, where)
	if err != nil {
		return suppressionEntry{}, err
	}
	if err := m.allow(keyPaths, keyReason); err != nil {
		return suppressionEntry{}, fmt.Errorf("%s: %v", where, err)
	}
	var out suppressionEntry
	v := m.get(keyPaths)
	if v == nil {
		return suppressionEntry{}, fmt.Errorf("%s: paths is required", where)
	}
	patterns, err := stringOrList(v, where+".paths")
	if err != nil {
		return suppressionEntry{}, err
	}
	if out.paths, err = rule.NewGlobs(patterns); err != nil {
		return suppressionEntry{}, fmt.Errorf("%s.paths: %v", where, err)
	}
	if out.reason, err = requiredReason(m, where); err != nil {
		return suppressionEntry{}, err
	}
	return out, nil
}

func requiredReason(m mapping, where string) (string, error) {
	v := m.get(keyReason)
	if v == nil {
		return "", fmt.Errorf("%s: reason is required so the decision stays inspectable", where)
	}
	reason, err := scalarString(v, where+".reason")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(reason) == "" {
		return "", fmt.Errorf("%s.reason: is blank", where)
	}
	return reason, nil
}

// distribution translates a Pattern file into a Pattern: local Rule
// IDs are qualified with the Pattern namespace, Modules carry
// descriptions and suggested paths, and every Rule names only listed
// Modules.
func (d *document) distribution(extensions []rule.PatternExtension) (rule.Pattern, error) {
	h := d.pattern
	fail := func(format string, args ...any) (rule.Pattern, error) {
		return rule.Pattern{}, fmt.Errorf("%s: %s", d.source, fmt.Sprintf(format, args...))
	}
	if _, err := rule.NewPatternReference(h.namespace, h.name, h.version); err != nil {
		return fail("%v", err)
	}
	modules := make([]rule.PatternModule, 0, len(d.modules))
	declared := map[rule.ModuleName]bool{}
	for _, m := range d.modules {
		pm, err := rule.NewPatternModule(m.name, m.description, m.paths)
		if err != nil {
			return fail("%s: %v", m.where, err)
		}
		modules = append(modules, pm)
		declared[m.name] = true
	}
	var rules []rule.Rule
	for _, e := range d.rules {
		if e.assertion == "" {
			return fail("%s: carries no assertion; a pattern distributes Rules and cannot override", e.where)
		}
		id, err := rule.NewID(e.id)
		if err != nil {
			return fail("%s: %v", e.where, err)
		}
		if id.Namespace() != "" {
			return fail("%s: rule ids inside a pattern are local; the loader qualifies them with namespace %q", e.where, h.namespace)
		}
		e.id = h.namespace + ":" + id.Local()
		r, err := e.build(vocab.UbiquitousLanguage{}, declared)
		if err != nil {
			return fail("%v", err)
		}
		rules = append(rules, r)
	}
	p, err := rule.NewPattern(rule.PatternSpec{
		Namespace:     h.namespace,
		Name:          h.name,
		Version:       h.version,
		Documentation: h.documentation,
		Coverage:      h.coverage,
		Modules:       modules,
		Rules:         rules,
		Extensions:    extensions,
	})
	if err != nil {
		return fail("%v", err)
	}
	return p, nil
}

// repository translates a repository ruleset: extended Patterns are
// resolved and bound, their Rules re-expanded against the recorded
// language, Overrides applied, and local Rules added.
func (d *document) repository(lang vocab.UbiquitousLanguage, available []rule.Pattern) (rule.Configured, error) {
	fail := func(format string, args ...any) (rule.Configured, error) {
		return rule.Configured{}, fmt.Errorf("%s: %s", d.source, fmt.Sprintf(format, args...))
	}
	byRef := map[string]rule.Pattern{}
	for _, p := range available {
		byRef[p.Reference().String()] = p
	}

	var modules []rule.Module
	moduleIndex := map[rule.ModuleName]int{}
	addModule := func(m rule.Module, where string) error {
		if i, dup := moduleIndex[m.Name()]; dup {
			if sameGlobs(modules[i].Paths(), m.Paths()) {
				return nil
			}
			return fmt.Errorf("%s: module %q is already declared with different paths", where, m.Name())
		}
		moduleIndex[m.Name()] = len(modules)
		modules = append(modules, m)
		return nil
	}

	var rules []rule.Rule
	patternRules := map[string]int{}
	var extensions []rule.ConfiguredExtension
	for _, ext := range d.extends {
		p, ok := byRef[ext.ref.String()]
		if !ok {
			return fail("%s: pattern %s is not available; known patterns: %s", ext.where, ext.ref, knownPatterns(available))
		}
		bound, err := p.Bind(ext.bindings)
		if err != nil {
			return fail("%s: %v", ext.where, err)
		}
		for _, m := range bound {
			if err := addModule(m, ext.where+".bind"); err != nil {
				return fail("%v", err)
			}
		}
		for _, r := range p.Rules() {
			r, err := r.Reexpand(lang)
			if err != nil {
				return fail("%s: %v", ext.where, err)
			}
			q := r.ID().Qualified()
			if _, dup := patternRules[q]; dup {
				return fail("%s: rule %s is distributed by two extended patterns", ext.where, q)
			}
			patternRules[q] = len(rules)
			rules = append(rules, r)
		}
		for _, e := range p.Extensions() {
			extensions = append(extensions, rule.ConfiguredExtension{Pattern: p.Reference(), Extension: e})
		}
	}
	for _, m := range d.modules {
		mod, err := rule.NewModule(m.name, m.description, m.paths)
		if err != nil {
			return fail("%s: %v", m.where, err)
		}
		if _, dup := moduleIndex[m.name]; dup {
			return fail("%s: module %q is already bound by an extended pattern", m.where, m.name)
		}
		if err := addModule(mod, m.where); err != nil {
			return fail("%v", err)
		}
	}
	declared := map[rule.ModuleName]bool{}
	for _, m := range modules {
		declared[m.Name()] = true
	}

	seen := map[string]bool{}
	for _, e := range d.rules {
		id, err := rule.NewID(e.id)
		if err != nil {
			return fail("%s: %v", e.where, err)
		}
		q := id.Qualified()
		if e.assertion == "" {
			i, ok := patternRules[q]
			if !ok {
				return fail("%s: carries no assertion, so it is an override, but no extended pattern distributes rule %s; give a new Rule one assertion key (%s)",
					e.where, q, strings.Join(rule.AssertionKeys(), ", "))
			}
			if seen[q] {
				return fail("%s: rule %s is overridden twice", e.where, q)
			}
			seen[q] = true
			overridden, err := e.applyOverride(rules[i])
			if err != nil {
				return fail("%v", err)
			}
			rules[i] = overridden
			continue
		}
		if _, ok := patternRules[q]; ok {
			return fail("%s: rule %s is distributed by an extended pattern; to change what it asserts, disable it with a reason and add a local Rule under a new ID",
				e.where, q)
		}
		if seen[q] {
			return fail("%s: duplicate rule id %q", e.where, q)
		}
		seen[q] = true
		r, err := e.build(lang, declared)
		if err != nil {
			return fail("%v", err)
		}
		rules = append(rules, r)
	}
	return rule.Configured{
		Rules:      rules,
		Modules:    modules,
		Languages:  d.runtime,
		Scan:       d.scan,
		Extensions: extensions,
	}, nil
}

func knownPatterns(available []rule.Pattern) string {
	if len(available) == 0 {
		return "none"
	}
	names := make([]string, 0, len(available))
	for _, p := range available {
		names = append(names, p.Reference().String())
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

func sameGlobs(a, b []rule.Glob) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].String() != b[i].String() {
			return false
		}
	}
	return true
}

// applyOverride merges the entry's adoption decisions onto a Pattern
// Rule. An Override carries no assertion, no description, and no
// applicability of its own.
func (e ruleEntry) applyOverride(r rule.Rule) (rule.Rule, error) {
	for _, bad := range []struct {
		set  bool
		key  string
		hint string
	}{
		{e.description != "", keyDescription, "a pattern rule keeps its own description"},
		{e.onPresent, keyOn, "a pattern rule keeps its own modules; use exclude to narrow it"},
		{len(e.files) > 0, keyFiles, "a pattern rule keeps its own files; use exclude to narrow it"},
		{e.withPresent, keyWith, "a pattern rule keeps its own parameters"},
	} {
		if bad.set {
			return rule.Rule{}, fmt.Errorf("%s: an override does not accept %s (%s); to change what the rule asserts, disable it with a reason and add a local Rule under a new ID",
				e.where, bad.key, bad.hint)
		}
	}
	if e.severity == "" && e.disable == nil && e.exclude == nil && e.suppress == nil {
		return rule.Rule{}, fmt.Errorf("%s: an override changes something: severity, disable, exclude, or suppress", e.where)
	}
	return e.adopt(r)
}

// adopt applies severity, exclude, suppress, and disable, in that
// order, to a Rule.
func (e ruleEntry) adopt(r rule.Rule) (rule.Rule, error) {
	if e.severity != "" {
		s, err := rule.ParseSeverity(e.severity)
		if err != nil {
			return rule.Rule{}, fmt.Errorf("%s.severity: %v", e.where, err)
		}
		if r, err = r.WithSeverity(s); err != nil {
			return rule.Rule{}, fmt.Errorf("%s: %v", e.where, err)
		}
	}
	if e.exclude != nil {
		ex, err := rule.NewExclusion(e.exclude.paths, e.exclude.modules, e.exclude.reason)
		if err != nil {
			return rule.Rule{}, fmt.Errorf("%s.exclude: %v", e.where, err)
		}
		r = r.Exclude(ex)
	}
	if e.suppress != nil {
		su, err := rule.NewSuppression(e.suppress.paths, e.suppress.reason)
		if err != nil {
			return rule.Rule{}, fmt.Errorf("%s.suppress: %v", e.where, err)
		}
		r = r.Suppress(su)
	}
	if e.disable != nil {
		dis, err := rule.NewDisablement(*e.disable)
		if err != nil {
			return rule.Rule{}, fmt.Errorf("%s.disable: %v", e.where, err)
		}
		r = r.Disable(dis)
	}
	return r, nil
}

// build constructs the Rule an entry with an assertion spells, then
// applies its own adoption decisions.
func (e ruleEntry) build(lang vocab.UbiquitousLanguage, declared map[rule.ModuleName]bool) (rule.Rule, error) {
	t, ok := rule.TypeOfAssertionKey(e.assertion)
	if !ok {
		return rule.Rule{}, fmt.Errorf("%s: unknown assertion %q", e.where, e.assertion)
	}
	if e.withPresent && t != rule.TypeExtension {
		return rule.Rule{}, fmt.Errorf("%s: with belongs to uses; %s carries its parameters under %s", e.where, e.assertion, e.assertion)
	}
	if len(e.files) > 0 && !t.AcceptsFiles() {
		return rule.Rule{}, fmt.Errorf("%s: %s does not accept files; it judges whole modules", e.where, e.assertion)
	}
	for _, m := range e.on {
		if !declared[m] {
			return rule.Rule{}, fmt.Errorf("%s.on: module %q is not declared", e.where, m)
		}
	}
	spec := rule.Spec{ID: e.id, Type: t, Claim: e.description, Severity: e.severity}
	var err error
	switch t.Scope() {
	case rule.ScopeModules:
		if !e.onPresent {
			return rule.Rule{}, fmt.Errorf("%s: %s requires on (the module or modules it judges)", e.where, e.assertion)
		}
		if spec.Applicability, err = rule.ModuleApplicability(e.on, e.files...); err != nil {
			return rule.Rule{}, fmt.Errorf("%s.on: %v", e.where, err)
		}
	case rule.ScopeOneModule:
		if len(e.on) != 1 {
			return rule.Rule{}, fmt.Errorf("%s: %s requires on naming exactly one module", e.where, e.assertion)
		}
		if spec.Applicability, err = rule.RepositoryApplicability(); err != nil {
			return rule.Rule{}, fmt.Errorf("%s: %v", e.where, err)
		}
	case rule.ScopeRepository:
		if e.onPresent {
			return rule.Rule{}, fmt.Errorf("%s: %s names modules itself, so it has no on", e.where, e.assertion)
		}
		if spec.Applicability, err = rule.RepositoryApplicability(); err != nil {
			return rule.Rule{}, fmt.Errorf("%s: %v", e.where, err)
		}
	case rule.ScopeModulesOrRepository:
		if e.onPresent {
			spec.Applicability, err = rule.ModuleApplicability(e.on, e.files...)
		} else {
			spec.Applicability, err = rule.RepositoryApplicability(e.files...)
		}
		if err != nil {
			return rule.Rule{}, fmt.Errorf("%s: %v", e.where, err)
		}
	}
	where := e.where + "." + e.assertion
	switch t {
	case rule.TypeConsumes:
		spec.Params, err = parseImports(e.assertNode, where)
	case rule.TypeStructure:
		spec.Params, spec.Expansion, err = parseStructure(e.assertNode, where, lang)
	case rule.TypeNaming:
		spec.Params, err = parseNaming(e.assertNode, where)
	case rule.TypeLayers:
		spec.Params, err = parseLayers(e.assertNode, where)
	case rule.TypeProtected:
		spec.Params, err = parseImportedBy(e.assertNode, where, e.on[0])
	case rule.TypeIndependence:
		spec.Params, err = parseIndependent(e.assertNode, where)
	case rule.TypeAcyclic:
		spec.Params, err = parseAcyclic(e.assertNode, where)
	case rule.TypeInvariants:
		spec.Params, err = parseInvariants(e.assertNode, where)
	case rule.TypeContent:
		spec.Params, err = parseContent(e.assertNode, where)
	case rule.TypeExtension:
		spec.Params, err = parseUses(e.assertNode, where, e.with)
	}
	if err != nil {
		return rule.Rule{}, err
	}
	r, err := rule.New(spec)
	if err != nil {
		return rule.Rule{}, fmt.Errorf("%s: %v", e.where, err)
	}
	for _, m := range r.ReferencedModules() {
		if !declared[m] {
			return rule.Rule{}, fmt.Errorf("%s: names module %q, which is not declared", e.where, m)
		}
	}
	return e.adopt(r)
}

func parseImports(n *yaml.Node, where string) (rule.Params, error) {
	m, err := asMapping(n, where)
	if err != nil {
		return nil, err
	}
	if err := m.allow(keyInternal, keyExternal, keyStdlib); err != nil {
		return nil, fmt.Errorf("%s: %v", where, err)
	}
	params := rule.ConsumesParams{}
	if v := m.get(keyInternal); v != nil {
		names, err := stringList(v, where+".internal")
		if err != nil {
			return nil, err
		}
		moduleNames := make([]rule.ModuleName, 0, len(names))
		for _, name := range names {
			mn, err := rule.NewModuleName(name)
			if err != nil {
				return nil, fmt.Errorf("%s.internal: %v", where, err)
			}
			moduleNames = append(moduleNames, mn)
		}
		allow, err := rule.NewAllowList(moduleNames...)
		if err != nil {
			return nil, fmt.Errorf("%s.internal: %v", where, err)
		}
		params.Internal = &allow
	}
	policy := func(key string) (rule.ImportPolicy, error) {
		s := ""
		if v := m.get(key); v != nil {
			var err error
			if s, err = scalarString(v, where+"."+key); err != nil {
				return "", err
			}
		}
		p, err := rule.ParseImportPolicy(s)
		if err != nil {
			return "", fmt.Errorf("%s.%s: %v", where, key, err)
		}
		return p, nil
	}
	if params.External, err = policy(keyExternal); err != nil {
		return nil, err
	}
	if params.Stdlib, err = policy(keyStdlib); err != nil {
		return nil, err
	}
	return params, nil
}

func parseStructure(n *yaml.Node, where string, lang vocab.UbiquitousLanguage) (rule.Params, *rule.Expansion, error) {
	m, err := asMapping(n, where)
	if err != nil {
		return nil, nil, err
	}
	if err := m.allow(keyRequire, keyForbid, keyEach); err != nil {
		return nil, nil, fmt.Errorf("%s: %v", where, err)
	}
	lists := map[string][]string{}
	for _, key := range []string{keyRequire, keyForbid} {
		if v := m.get(key); v != nil {
			if lists[key], err = stringList(v, where+"."+key); err != nil {
				return nil, nil, err
			}
		}
	}
	if v := m.get(keyEach); v != nil {
		source, err := scalarString(v, where+".each")
		if err != nil {
			return nil, nil, err
		}
		expansion, err := rule.NewExpansion(source, lists[keyRequire], lists[keyForbid])
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %v", where, err)
		}
		params, err := expansion.Resolve(lang)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %v", where, err)
		}
		return params, &expansion, nil
	}
	require, err := rule.NewGlobs(lists[keyRequire])
	if err != nil {
		return nil, nil, fmt.Errorf("%s.require: %v", where, err)
	}
	forbid, err := rule.NewGlobs(lists[keyForbid])
	if err != nil {
		return nil, nil, fmt.Errorf("%s.forbid: %v", where, err)
	}
	return rule.StructureParams{Require: require, Forbid: forbid}, nil, nil
}

func parseNaming(n *yaml.Node, where string) (rule.Params, error) {
	var spelled string
	switch n.Kind {
	case yaml.ScalarNode:
		s, err := scalarString(n, where)
		if err != nil {
			return nil, err
		}
		spelled = s
	case yaml.MappingNode:
		m, err := asMapping(n, where)
		if err != nil {
			return nil, err
		}
		if err := m.allow(keyCase); err != nil {
			return nil, fmt.Errorf("%s: %v", where, err)
		}
		v := m.get(keyCase)
		if v == nil {
			return nil, fmt.Errorf("%s: case is required", where)
		}
		if spelled, err = scalarString(v, where+".case"); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("%s: expected a case spec such as snake_case, or an object with case", where)
	}
	caseSpec, err := rule.NewCaseSpec(spelled)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", where, err)
	}
	return rule.NamingParams{Case: caseSpec}, nil
}

func parseLayers(n *yaml.Node, where string) (rule.Params, error) {
	layers, err := moduleNameList(n, where)
	if err != nil {
		return nil, err
	}
	return rule.LayersParams{Layers: layers}, nil
}

func parseImportedBy(n *yaml.Node, where string, protected rule.ModuleName) (rule.Params, error) {
	allow, err := moduleNameList(n, where)
	if err != nil {
		return nil, err
	}
	return rule.ProtectedParams{Module: protected, Allow: allow}, nil
}

func parseIndependent(n *yaml.Node, where string) (rule.Params, error) {
	patterns, err := stringList(n, where)
	if err != nil {
		return nil, err
	}
	globs, err := rule.NewGlobs(patterns)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", where, err)
	}
	return rule.IndependenceParams{Folders: globs}, nil
}

func parseAcyclic(n *yaml.Node, where string) (rule.Params, error) {
	switch n.Kind {
	case yaml.SequenceNode:
		modules, err := moduleNameList(n, where)
		if err != nil {
			return nil, err
		}
		if len(modules) < 2 {
			return nil, fmt.Errorf("%s: a cycle needs at least two modules; use {} for every declared module", where)
		}
		return rule.AcyclicParams{Modules: modules}, nil
	case yaml.MappingNode:
		if len(n.Content) != 0 {
			return nil, fmt.Errorf("%s: expected a list of modules or {} for every declared module", where)
		}
		return rule.AcyclicParams{}, nil
	default:
		return nil, fmt.Errorf("%s: expected a list of modules or {} for every declared module", where)
	}
}

func parseInvariants(n *yaml.Node, where string) (rule.Params, error) {
	m, err := asMapping(n, where)
	if err != nil {
		return nil, err
	}
	if err := m.allow(keyClosed); err != nil {
		return nil, fmt.Errorf("%s: %v", where, err)
	}
	params := rule.InvariantsParams{}
	if v := m.get(keyClosed); v != nil {
		if params.Closed, err = boolValue(v, where+".closed"); err != nil {
			return nil, err
		}
	}
	return params, nil
}

func parseContent(n *yaml.Node, where string) (rule.Params, error) {
	m, err := asMapping(n, where)
	if err != nil {
		return nil, err
	}
	if err := m.allow(keyForbid); err != nil {
		return nil, fmt.Errorf("%s: %v", where, err)
	}
	v := m.get(keyForbid)
	if v == nil {
		return nil, fmt.Errorf("%s: forbid is required (a regular expression no line may match)", where)
	}
	pattern, err := scalarString(v, where+".forbid")
	if err != nil {
		return nil, err
	}
	return rule.ContentParams{Forbid: pattern}, nil
}

func parseUses(n *yaml.Node, where string, with map[string]any) (rule.Params, error) {
	name, err := scalarString(n, where)
	if err != nil {
		return nil, err
	}
	return rule.ExtensionParams{Uses: name, With: with}, nil
}

func moduleNameList(n *yaml.Node, where string) ([]rule.ModuleName, error) {
	names, err := stringList(n, where)
	if err != nil {
		return nil, err
	}
	out := make([]rule.ModuleName, 0, len(names))
	for _, name := range names {
		mn, err := rule.NewModuleName(name)
		if err != nil {
			return nil, fmt.Errorf("%s: %v", where, err)
		}
		out = append(out, mn)
	}
	return out, nil
}

// runtimeTargets are the one spelling of language targets the ruleset
// format accepts, under runtime in a repository ruleset and under
// pattern.coverage in a Pattern file alike; each resolves to a
// published Language.
var runtimeTargetLanguages = map[string]rule.Language{
	"go": rule.LanguageGo,
	"ts": rule.LanguageTypeScript,
	"py": rule.LanguagePython,
}

func runtimeTargets() []string { return []string{"go", "ts", "py"} }

// translateTargets resolves a list of runtime targets to Languages,
// rejecting unknown spellings and repeats; where names the key the
// error is about.
func translateTargets(where string, targets []string) ([]rule.Language, error) {
	out := make([]rule.Language, 0, len(targets))
	seen := map[string]bool{}
	for _, target := range targets {
		l, ok := runtimeTargetLanguages[target]
		if !ok {
			return nil, fmt.Errorf("%s target %q: not one of %s", where, target, strings.Join(runtimeTargets(), ", "))
		}
		if seen[target] {
			return nil, fmt.Errorf("%s target %q: listed twice", where, target)
		}
		seen[target] = true
		out = append(out, l)
	}
	return out, nil
}

// mapping is a strict view over a YAML mapping node: keys in document
// order, duplicate keys rejected, unknown keys reported by name.
type mapping struct {
	where string
	order []string
	nodes map[string]*yaml.Node
}

func asMapping(n *yaml.Node, where string) (mapping, error) {
	if n == nil || n.Kind != yaml.MappingNode {
		return mapping{}, fmt.Errorf("%s: expected an object", where)
	}
	m := mapping{where: where, nodes: map[string]*yaml.Node{}}
	for i := 0; i+1 < len(n.Content); i += 2 {
		k := n.Content[i]
		if k.Kind != yaml.ScalarNode {
			return mapping{}, fmt.Errorf("%s: keys must be plain strings", where)
		}
		if _, dup := m.nodes[k.Value]; dup {
			return mapping{}, fmt.Errorf("%s: key %q appears twice", where, k.Value)
		}
		m.order = append(m.order, k.Value)
		m.nodes[k.Value] = n.Content[i+1]
	}
	return m, nil
}

func (m mapping) keys() []string { return m.order }

func (m mapping) get(key string) *yaml.Node { return m.nodes[key] }

// allow rejects the first key, in document order, outside the accepted
// set.
func (m mapping) allow(accepted ...string) error {
	ok := map[string]bool{}
	for _, k := range accepted {
		ok[k] = true
	}
	for _, k := range m.order {
		if !ok[k] {
			sorted := append([]string(nil), accepted...)
			sort.Strings(sorted)
			return fmt.Errorf("unknown key %q (accepted: %s)", k, strings.Join(sorted, ", "))
		}
	}
	return nil
}

func scalarString(n *yaml.Node, where string) (string, error) {
	if n.Kind != yaml.ScalarNode || n.Tag == "!!null" {
		return "", fmt.Errorf("%s: expected a string", where)
	}
	if n.Tag != "!!str" {
		return "", fmt.Errorf("%s: expected a string, got %s", where, strings.TrimPrefix(n.Tag, "!!"))
	}
	return n.Value, nil
}

func boolValue(n *yaml.Node, where string) (bool, error) {
	if n.Kind != yaml.ScalarNode || n.Tag != "!!bool" {
		return false, fmt.Errorf("%s: expected true or false", where)
	}
	var b bool
	if err := n.Decode(&b); err != nil {
		return false, fmt.Errorf("%s: %v", where, err)
	}
	return b, nil
}

func stringList(n *yaml.Node, where string) ([]string, error) {
	if n.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("%s: expected a list of strings", where)
	}
	out := make([]string, 0, len(n.Content))
	for i, item := range n.Content {
		s, err := scalarString(item, fmt.Sprintf("%s[%d]", where, i))
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

// stringOrList expands the one-or-many sugar: a scalar is a one-item
// list.
func stringOrList(n *yaml.Node, where string) ([]string, error) {
	if n.Kind == yaml.ScalarNode {
		s, err := scalarString(n, where)
		if err != nil {
			return nil, err
		}
		return []string{s}, nil
	}
	list, err := stringList(n, where)
	if err != nil {
		return nil, fmt.Errorf("%s: expected a string or a list of strings", where)
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("%s: list is empty", where)
	}
	return list, nil
}

// DiscoverPath locates the ruleset file: from a starting directory
// upward to the filesystem root.
func DiscoverPath(start, filename string) (string, error) {
	abs, err := filepath.Abs(start)
	if err != nil {
		return "", fmt.Errorf("ruleset discovery: %w", err)
	}
	if info, err := os.Stat(abs); err == nil && !info.IsDir() {
		abs = filepath.Dir(abs)
	}
	dir := abs
	for {
		candidate := filepath.Join(dir, filename)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no %s found in %s or any parent directory", filename, abs)
		}
		dir = parent
	}
}
