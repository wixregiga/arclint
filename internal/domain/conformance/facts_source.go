package conformance

import (
	"fmt"
	"path"
	"slices"
	"sort"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/rule"
)

// membership resolves Zone membership over the Observations once,
// for every evaluator.
type membership struct {
	names    []rule.ZoneName // sorted
	declared map[rule.ZoneName]bool
	files    []string // sorted repo-relative paths
	// ownedFolders holds every folder above an observed file that a Zone
	// contains.
	ownedFolders map[string]bool
	fileZones    map[string][]rule.ZoneName // sorted per file
	zoneFiles    map[rule.ZoneName][]string // path order
	dirZones     map[string][]rule.ZoneName // sorted per directory
}

func newMembership(zones []rule.Zone, obs Observations) (membership, error) {
	mem := membership{
		declared:     map[rule.ZoneName]bool{},
		ownedFolders: map[string]bool{},
		fileZones:    map[string][]rule.ZoneName{},
		zoneFiles:    map[rule.ZoneName][]string{},
		dirZones:     map[string][]rule.ZoneName{},
	}
	byName := map[rule.ZoneName]rule.Zone{}
	for _, m := range zones {
		if mem.declared[m.Name()] {
			return membership{}, fmt.Errorf("conformance: duplicate zone %q", m.Name())
		}
		mem.declared[m.Name()] = true
		byName[m.Name()] = m
		mem.names = append(mem.names, m.Name())
	}
	sort.Slice(mem.names, func(i, j int) bool { return mem.names[i] < mem.names[j] })
	folders := map[string]bool{}

	dirSets := map[string]map[rule.ZoneName]bool{}
	for _, f := range obs.Files() {
		mem.files = append(mem.files, f.Path)
		mem.fileZones[f.Path] = nil // Presence records observation even without a Zone.
		for _, name := range mem.names {
			if byName[name].Contains(f.Path) {
				mem.fileZones[f.Path] = append(mem.fileZones[f.Path], name)
				mem.zoneFiles[name] = append(mem.zoneFiles[name], f.Path)
			}
		}
		d := path.Dir(f.Path)
		set := dirSets[d]
		if set == nil {
			set = map[rule.ZoneName]bool{}
			dirSets[d] = set
		}
		for _, m := range mem.fileZones[f.Path] {
			set[m] = true
		}
		for folder := path.Dir(f.Path); folder != "." && !folders[folder]; folder = path.Dir(folder) {
			folders[folder] = true
		}
	}
	for folder := range folders {
		for _, m := range zones {
			if m.Contains(folder) {
				mem.ownedFolders[folder] = true
				break
			}
		}
	}
	for d, set := range dirSets {
		names := make([]rule.ZoneName, 0, len(set))
		for m := range set {
			names = append(names, m)
		}
		sort.Slice(names, func(i, j int) bool { return names[i] < names[j] })
		mem.dirZones[d] = names
	}
	return mem, nil
}

// targetMembership resolves the declared Zones an internal import lands
// in: file-granular targets through the file's own membership,
// package-granular targets through the directory union.
func (m membership) targetMembership(imp Import) ([]rule.ZoneName, bool) {
	kind, target := imp.Target()
	switch kind {
	case importTargetFile:
		zones, observed := m.fileZones[target]
		return zones, observed
	case importTargetDirectory:
		zones, observed := m.dirZones[target]
		return zones, observed
	}
	return nil, false
}

// factsSource owns the collected input and shared query indexes. Only input
// preparation can access it; evaluators receive Facts and cannot request a
// broader view. Indexes are implementation details, not additional fact models.
type factsSource struct {
	observations Observations
	membership   membership
	dependencies *dependencyIndex
	files        map[string]ObservedFile
}

func newFactsSource(zones []rule.Zone, obs Observations) (*factsSource, error) {
	mem, err := newMembership(zones, obs)
	if err != nil {
		return nil, err
	}
	files := make(map[string]ObservedFile, len(obs.files))
	for _, file := range obs.files {
		files[file.Path] = file
	}
	return &factsSource{
		observations: obs, membership: mem,
		dependencies: newDependencyIndex(obs, mem), files: files,
	}, nil
}

// forCarriers prepares what a Rule checking recorded contracts selects,
// with its own facts and the imports, to locate the declaration
// carrying each contract: a Go test carrying an assertion takes T under
// the name its file imports testing by. It never evaluates the Rule.
func (s *factsSource) forCarriers(r rule.Rule) Facts {
	required := append([]rule.Fact(nil), r.Enforcement().Facts()...)
	if !slices.Contains(required, rule.FactImports) {
		required = append(required, rule.FactImports)
	}
	f := s.prepare(required, r.Scope().WouldSelectFile, r.Scope().ExcludedFile)
	f.constraint, f.scope = r.Type(), r.Scope()
	return f
}

// forRule is the sole preparation path for native and extension evaluators.
// Scope chooses file access; graph constraints additionally require repository
// edges to judge Zone subjects. An excluded Zone remains graph evidence for
// other subjects, matching Scope's subject-exclusion semantics.
func (s *factsSource) forRule(r rule.Rule) Facts {
	f := s.prepare(r.Enforcement().Facts(), r.Scope().WouldSelectFile, r.Scope().ExcludedFile)
	f.constraint, f.scope = r.Type(), r.Scope()
	if f.required[rule.FactImports] {
		var zones []rule.ZoneName
		switch p := r.Params().(type) {
		case rule.LayersParams:
			zones = p.Layers
		case rule.ProtectedParams:
			zones = []rule.ZoneName{p.Zone}
		case rule.AcyclicParams:
			zones = p.Zones
			if len(zones) == 0 {
				zones = s.membership.names
			}
		default:
			return f
		}
		f.graphZones = make(map[rule.ZoneName]bool, len(zones))
		for _, zone := range zones {
			f.graphZones[zone] = true
		}
	}
	return f
}

func (s *factsSource) prepare(required []rule.Fact, selects func(string, []rule.ZoneName) bool, excluded func(string) bool) Facts {
	f := Facts{source: s, required: map[rule.Fact]bool{}, allowed: map[string]bool{}, excludeFile: excluded}
	for _, fact := range required {
		f.required[fact] = true
	}
	for _, path := range s.membership.files {
		if !selects(path, s.membership.fileZones[path]) {
			continue
		}
		if excluded(path) {
			f.excluded = append(f.excluded, path)
			continue
		}
		f.selected = append(f.selected, path)
		f.allowed[path] = true
	}
	return f
}

// Subject partitions are prepared once, independently from required classes:
// even a Rule requiring imports alone needs the identities it must evaluate.
func (f Facts) selectedFiles() (selected, excluded []string) {
	return append([]string(nil), f.selected...), append([]string(nil), f.excluded...)
}

func (f Facts) paths() []string { return append([]string(nil), f.selected...) }

func (f Facts) selectedZoneFiles(name rule.ZoneName) (selected, excluded []string) {
	if f.source == nil {
		return nil, nil
	}
	for _, path := range f.selected {
		if zoneIn(f.source.membership.fileZones[path], name) {
			selected = append(selected, path)
		}
	}
	for _, path := range f.excluded {
		if zoneIn(f.source.membership.fileZones[path], name) {
			excluded = append(excluded, path)
		}
	}
	return selected, excluded
}

// excludedAnalyzed counts the files the Rule's Exclusion removed from a
// context's scope (the Zone named for it, or the repository) that an
// analyzer read: files whose exclusion, not their language, keeps their
// facts from the Rule.
func (f Facts) excludedAnalyzed(zone rule.ZoneName) int {
	if f.source == nil {
		return 0
	}
	excluded := f.excluded
	if zone != "" {
		_, excluded = f.selectedZoneFiles(zone)
	}
	n := 0
	for _, path := range excluded {
		if facts, ok := f.source.observations.facts[path]; ok && facts.Language != "" {
			n++
		}
	}
	return n
}

func (f Facts) zoneNames() []rule.ZoneName {
	if f.source == nil {
		return nil
	}
	return append([]rule.ZoneName(nil), f.source.membership.names...)
}

func (f Facts) zoneMembers(name rule.ZoneName) []string {
	out := []string{}
	if f.source != nil {
		for _, path := range f.source.membership.zoneFiles[name] {
			// Structure judges a Zone's composition. A file excluded as a
			// subject still witnesses that a required member exists; it
			// remains unreadable through Read or FactsFor.
			if f.Contains(path) || (f.constraint == rule.TypeStructure && f.scope.SelectsZone(name)) {
				out = append(out, path)
			}
		}
	}
	return out
}

// Zone graph facts are supplied only for graph constraints. Their evidence can
// cross selected file boundaries without making those files readable.
func (f Facts) zoneEdges() ([]edge, error) {
	if f.graphZones == nil {
		return nil, fmt.Errorf("facts: Zone graph evidence was not prepared for constraint %q", f.constraint)
	}
	out := []edge{}
	for _, e := range f.source.dependencies.zoneEdges() {
		if !f.graphZones[e.from] && !f.graphZones[e.to] {
			continue
		}
		e.sourceZones = append([]rule.ZoneName(nil), e.sourceZones...)
		out = append(out, e)
	}
	return out, nil
}

// independentFolders supplies the observed sibling identities needed to judge
// independence. An excluded file can establish a Folder's existence without
// becoming readable or contributing its outgoing imports.
func (f Facts) independentFolders(p rule.IndependenceParams) ([]string, error) {
	if f.source == nil || f.constraint != rule.TypeIndependence || !f.required[rule.FactImports] {
		return nil, fmt.Errorf("facts: sibling Folder evidence was not prepared for constraint %q", f.constraint)
	}
	seen := map[string]bool{}
	var out []string
	for _, g := range p.Folders {
		n := len(strings.Split(g.String(), "/"))
		for _, file := range f.source.membership.files {
			parts := strings.Split(file, "/")
			if len(parts) <= n {
				continue
			}
			candidate := strings.Join(parts[:n], "/")
			if seen[candidate] || !g.Match(candidate) || f.zoneOwnsFolder(candidate) {
				continue
			}
			seen[candidate] = true
			out = append(out, candidate)
		}
	}
	sort.Strings(out)
	return out, nil
}

func (f Facts) zoneOwnsFolder(folder string) bool {
	return f.source != nil && f.source.membership.ownedFolders[folder]
}

// sameCodeZones identifies nesting from complete memberships, not a restricted
// view that could make two different Zones appear to contain the same code.
func (f Facts) sameCodeZones(a, b rule.ZoneName) bool {
	if len(f.graphZones) == 0 {
		return false
	}
	members := f.source.membership.zoneFiles
	within := func(inner, outer rule.ZoneName) bool {
		if len(members[inner]) == 0 {
			return false
		}
		for _, path := range members[inner] {
			if !zoneIn(f.source.membership.fileZones[path], outer) {
				return false
			}
		}
		return true
	}
	return within(a, b) || within(b, a)
}

func (f Facts) externalTestPackage(path string) bool {
	if len(f.graphZones) == 0 {
		return false
	}
	facts, ok := f.source.observations.facts[path]
	return ok && facts.Language == rule.LanguageGo && strings.HasSuffix(facts.Package, "_test")
}

// contextScope uses an exactly named Zone, then a uniquely equivalent spelling,
// otherwise all permitted files. Native domain checks and CLI Carriers share it.
func (f Facts) contextScope(name string) (rule.ZoneName, []string, error) {
	if f.source == nil {
		return "", nil, nil
	}
	spelled := ZonesNamedFor(name, f.zoneNames())
	switch len(spelled) {
	case 0:
		return "", f.paths(), nil
	case 1:
		return spelled[0], f.zoneMembers(spelled[0]), nil
	default:
		return "", nil, fmt.Errorf("context %s: Zones %s all spell its name; keep one named for the context", name, quotedZones(spelled))
	}
}
