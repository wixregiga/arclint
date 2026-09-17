package conformance

import (
	"path"
	"sort"
	"sync"

	"github.com/wixregiga/arclint/internal/domain/rule"
)

// DependencyImport is a parsed dependency with both endpoints and the native
// resolver's precision. Direction is a view of this value, not another fact.
type DependencyImport struct {
	Import
	SourcePath  string
	SourceZones []rule.ZoneName
	TargetZones []rule.ZoneName
	// TargetObserved records a resolved file or directory present in observations,
	// independently of whether any declared Zone contains it.
	TargetObserved bool
}

func newDependencyImport(source string, imp Import, mem membership) DependencyImport {
	targets, observed := mem.targetMembership(imp)
	return DependencyImport{
		Import: imp, SourcePath: source,
		SourceZones: append([]rule.ZoneName{}, mem.fileZones[source]...),
		TargetZones: append([]rule.ZoneName{}, targets...), TargetObserved: observed,
	}
}

type dependencyRef struct {
	source  string
	ordinal int
}

// The index stores references into the observations, not a reconstructed
// repository export. It is built on first use and shared for the check.
type dependencyIndex struct {
	observations Observations
	membership   membership
	once         sync.Once
	refs         []dependencyRef
	outgoing     map[string][]int
	incomingFile map[string][]int
	incomingDir  map[string][]int
	zoneOnce     sync.Once
	edges        []edge
}

func newDependencyIndex(obs Observations, mem membership) *dependencyIndex {
	return &dependencyIndex{observations: obs, membership: mem}
}

func (index *dependencyIndex) ensure() {
	index.once.Do(func() {
		index.outgoing = map[string][]int{}
		index.incomingFile = map[string][]int{}
		index.incomingDir = map[string][]int{}
		for _, file := range index.observations.files {
			facts, ok := index.observations.facts[file.Path]
			if !ok || !facts.Supports(rule.FactImports) {
				continue
			}
			for ordinal, imp := range facts.Imports {
				id := len(index.refs)
				index.refs = append(index.refs, dependencyRef{file.Path, ordinal})
				index.outgoing[file.Path] = append(index.outgoing[file.Path], id)
				kind, target := imp.Target()
				switch kind {
				case importTargetFile:
					index.incomingFile[target] = append(index.incomingFile[target], id)
				case importTargetDirectory:
					index.incomingDir[target] = append(index.incomingDir[target], id)
				}
			}
		}
	})
}

func (index *dependencyIndex) importAt(id int) DependencyImport {
	ref := index.refs[id]
	facts, _ := index.observations.facts[ref.source]
	return newDependencyImport(ref.source, facts.Imports[ref.ordinal], index.membership)
}

func (index *dependencyIndex) zoneEdges() []edge {
	index.zoneOnce.Do(func() {
		index.ensure()
		for id := range index.refs {
			d := index.importAt(id)
			if d.Class != ImportInternal {
				continue
			}
			for _, to := range d.TargetZones {
				if len(d.SourceZones) == 0 {
					index.edges = append(index.edges, edge{to: to, path: d.SourcePath, line: d.Line, importPath: d.Path, sourceZones: d.SourceZones})
				}
				for _, from := range d.SourceZones {
					if from != to {
						index.edges = append(index.edges, edge{from: from, to: to, path: d.SourcePath, line: d.Line, importPath: d.Path, sourceZones: d.SourceZones})
					}
				}
			}
		}
	})
	return index.edges
}

// involving uses adjacency indexes, never a repository-wide scan per subject.
// A directory match remains package evidence, never an exact file dependency.
func (index *dependencyIndex) involving(file string) []int {
	index.ensure()
	ids := append([]int(nil), index.outgoing[file]...)
	ids = append(ids, index.incomingFile[file]...)
	ids = append(ids, index.incomingDir[path.Dir(file)]...)
	sort.Ints(ids)
	return ids
}
