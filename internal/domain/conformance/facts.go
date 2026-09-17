package conformance

import (
	"fmt"

	"github.com/wixregiga/arclint/internal/domain/rule"
)

// Facts is the read-only information prepared for one consumer. Every evaluator
// receives this same interface. Selection, required classes, dependency meaning,
// and file access are decided here, never by an evaluator or adapter.
//
// A referenced dependency endpoint is evidence, not permission to query that
// file. Facts has no operation that broadens its access or exposes its source.
// The zero value supplies no information or access.
type Facts struct {
	source      *factsSource
	required    map[rule.Fact]bool
	selected    []string
	excluded    []string
	allowed     map[string]bool
	excludeFile func(string) bool
	graphZones  map[rule.ZoneName]bool
	constraint  rule.Type
	scope       rule.Scope
}

// NewFacts prepares a Rule's input from validated observations. Run shares the
// source and its indexes across Rules instead of rebuilding them for each call.
func NewFacts(r rule.Rule, zones []rule.Zone, obs Observations) (Facts, error) {
	if err := r.Validate(); err != nil {
		return Facts{}, fmt.Errorf("facts: %w", err)
	}
	source, err := newFactsSource(zones, obs)
	if err != nil {
		return Facts{}, err
	}
	if _, err := validRules([]rule.Rule{r}, source.membership); err != nil {
		return Facts{}, err
	}
	return source.forRule(r), nil
}

// NewInspectionFacts prepares explicitly requested repository inspection facts.
// Only callers holding the collected observations can construct this view;
// a Rule's supplied Facts cannot broaden itself into an inspection view.
func NewInspectionFacts(zones []rule.Zone, obs Observations, required ...rule.Fact) (Facts, error) {
	for _, fact := range required {
		switch fact {
		case rule.FactFileTree, rule.FactImports, rule.FactDeclarations, rule.FactCalls:
		default:
			return Facts{}, fmt.Errorf("inspection facts: unknown class %q", fact)
		}
	}
	source, err := newFactsSource(zones, obs)
	if err != nil {
		return Facts{}, err
	}
	return source.prepare(required, func(string, []rule.ZoneName) bool { return true }, func(string) bool { return false }), nil
}

// Files lists the selected files when the file-tree fact was requested.
func (f Facts) Files() []ObservedFile {
	out := []ObservedFile{}
	if f.required[rule.FactFileTree] {
		for _, path := range f.selected {
			out = append(out, f.source.files[path])
		}
	}
	return out
}

// Contains reports whether path is selected, not merely an evidence endpoint.
func (f Facts) Contains(path string) bool { return f.allowed[path] }

// FactsFor returns detached language facts for a selected file, limited to the
// requested classes. Capability flags and parse failure remain separate: a
// failed parse must not become an unsupported language or an observed empty file.
func (f Facts) FactsFor(path string) (LanguageFacts, bool) {
	if !f.Contains(path) {
		return LanguageFacts{}, false
	}
	facts, ok := f.source.observations.FactsFor(path)
	if !ok {
		return LanguageFacts{}, false
	}
	facts.ImportsAvailable = f.required[rule.FactImports] && facts.ImportsAvailable
	facts.DeclarationsAvailable = f.required[rule.FactDeclarations] && facts.DeclarationsAvailable
	facts.CallsAvailable = f.required[rule.FactCalls] && facts.CallsAvailable
	if !facts.Supports(rule.FactImports) {
		facts.Imports = nil
	}
	if !facts.Supports(rule.FactDeclarations) {
		facts.Declarations = nil
	}
	if !facts.Supports(rule.FactCalls) {
		facts.Calls = nil
	}
	return facts, true
}

// ImportsFor returns the available outgoing imports enriched by the shared
// membership resolution. Native checks and SDK conversion use these same values.
func (f Facts) ImportsFor(path string) []DependencyImport {
	out := []DependencyImport{}
	if !f.Contains(path) || !f.required[rule.FactImports] {
		return out
	}
	index := f.source.dependencies
	index.ensure()
	for _, id := range index.outgoing[path] {
		out = append(out, index.importAt(id))
	}
	return out
}

// DependenciesFor returns both directions of observed evidence involving a
// selected file. A directory target remains package evidence. Explicitly
// excluded import sources are omitted; unrelated imports are never traversable.
func (f Facts) DependenciesFor(subject string) []DependencyImport {
	out := []DependencyImport{}
	if !f.Contains(subject) || !f.required[rule.FactImports] {
		return out
	}
	index := f.source.dependencies
	previous := -1
	for _, id := range index.involving(subject) {
		if id == previous {
			continue
		}
		previous = id
		if f.excludeFile(index.refs[id].source) {
			continue
		}
		out = append(out, index.importAt(id))
	}
	return out
}

// AllowsFinding validates an extension's subject and reported evidence location.
// Outside the selected subject, only a supplied import's source line is valid.
func (f Facts) AllowsFinding(subject, location string, line int) bool {
	if !f.Contains(subject) {
		return false
	}
	if subject == location {
		return true
	}
	for _, d := range f.DependenciesFor(subject) {
		if d.SourcePath == location && d.Line == line {
			return true
		}
	}
	return false
}

// Read lends content only for a selected file. Incoming and outgoing dependency
// references do not grant access to their other endpoint.
func (f Facts) Read(path string) (string, error) {
	if !f.Contains(path) {
		return "", fmt.Errorf("%s is outside this rule's scope", path)
	}
	content := f.source.observations.content
	if content == nil {
		return "", fmt.Errorf("read %s: no content capability on observations", path)
	}
	data, err := content.Read(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	return data, nil
}

// ZoneOf returns memberships of a selected file only. Other endpoint
// memberships are carried by DependencyImport, never an unrestricted lookup.
func (f Facts) ZoneOf(path string) []rule.ZoneName {
	if !f.Contains(path) {
		return nil
	}
	return append([]rule.ZoneName(nil), f.source.membership.fileZones[path]...)
}

// Zones lists declared Zone names with their selected member files.
func (f Facts) Zones() map[rule.ZoneName][]string {
	out := map[rule.ZoneName][]string{}
	for _, name := range f.zoneNames() {
		selected, _ := f.selectedZoneFiles(name)
		out[name] = append([]string{}, selected...)
	}
	return out
}
