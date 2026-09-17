package conformance

import (
	"fmt"
	"sort"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/rule"
)

// evaluateConsumes judges every code file of the Rule's Zones
// against the import policy. Files in languages without produced facts
// evaluate unsupported; analysis failures evaluate failed.
func evaluateConsumes(r rule.Rule, supplied Facts) ([]Evaluation, error) {
	p, ok := r.Params().(rule.ConsumesParams)
	if !ok {
		return nil, fmt.Errorf("rule %s: consumes rule with %T params", r.ID(), r.Params())
	}
	var out []Evaluation
	for _, name := range sortedZones(r.Scope().Zones()) {
		if r.Scope().ExcludedZone(name) {
			subject, err := rule.ZoneSubject(name)
			if err != nil {
				return nil, fmt.Errorf("consumes: %w", err)
			}
			e, err := simpleEvaluation(r, subject, OutcomeNotApplicable)
			if err != nil {
				return nil, err
			}
			out = append(out, e)
			continue
		}
		selected, excluded := supplied.selectedZoneFiles(name)
		var err error
		out, err = appendNotApplicable(out, r, excluded)
		if err != nil {
			return nil, err
		}
		for _, f := range selected {
			language := rule.LanguageOf(f)
			if language == "" {
				continue // not a code file; imports say nothing about it
			}
			subject, err := rule.FileSubject(f)
			if err != nil {
				return nil, fmt.Errorf("consumes: %w", err)
			}
			if !r.Enforcement().SupportsLanguage(language) {
				e, err := simpleEvaluation(r, subject, OutcomeUnsupported)
				if err != nil {
					return nil, err
				}
				out = append(out, e)
				continue
			}
			facts, have := supplied.FactsFor(f)
			if !have || !facts.ImportsAvailable {
				e, err := simpleEvaluation(r, subject, OutcomeUnsupported)
				if err != nil {
					return nil, err
				}
				out = append(out, e)
				continue
			}
			if facts.ParseFailure != "" {
				e, err := simpleEvaluation(r, subject, OutcomeFailed)
				if err != nil {
					return nil, err
				}
				out = append(out, e)
				continue
			}
			vs, err := consumesViolations(r, p, name, subject, f, supplied.ImportsFor(f))
			if err != nil {
				return nil, err
			}
			e, err := completeEvaluation(r, subject, vs)
			if err != nil {
				return nil, err
			}
			out = append(out, e)
		}
	}
	return out, nil
}

func consumesViolations(r rule.Rule, p rule.ConsumesParams, self rule.ZoneName,
	subject rule.Subject, path string, imports []DependencyImport,
) ([]Violation, error) {
	var vs []Violation
	add := func(line int, message, remediation string) error {
		v, err := newViolation(r, subject, path, line, message, remediation)
		if err != nil {
			return err
		}
		vs = append(vs, v)
		return nil
	}
	for _, imp := range imports {
		switch imp.Class {
		case ImportStdlib:
			if p.Stdlib.Forbids() {
				if err := add(imp.Line,
					fmt.Sprintf("standard-library import %q is forbidden for Zone %q", imp.Path, self),
					"remove the import or allow standard-library imports for the Zone"); err != nil {
					return nil, err
				}
			}
		case ImportExternal:
			if p.External.Forbids() {
				if err := add(imp.Line,
					fmt.Sprintf("external import %q is forbidden for Zone %q", imp.Path, self),
					"remove the import or allow external imports for the Zone"); err != nil {
					return nil, err
				}
			}
		case ImportInternal:
			if p.Internal == nil {
				continue
			}
			targets := imp.TargetZones
			if zoneIn(targets, self) {
				continue // the Zone itself, never a violation
			}
			permitted := false
			for _, t := range targets {
				if p.Internal.Permits(t) {
					permitted = true
					break
				}
			}
			if permitted {
				continue
			}
			var message string
			switch {
			case imp.TargetDir == "" && imp.TargetFile == "":
				message = fmt.Sprintf("import %q is internal but does not resolve into the repository, and Zone %q restricts internal imports", imp.Path, self)
			case len(targets) == 0:
				message = fmt.Sprintf("import %q resolves to code outside any declared Zone, not allowed for Zone %q", imp.Path, self)
			default:
				message = fmt.Sprintf("import %q resolves to Zone(s) %s, not in the allow-list of Zone %q",
					imp.Path, quotedZones(targets), self)
			}
			if err := add(imp.Line, message,
				"add the target Zone to the allow-list or remove the import"); err != nil {
				return nil, err
			}
		case ImportUnknown, ImportCgo:
			// Unknown imports are the repository policy's concern
			// (observation diagnostics); cgo's "C" pseudo-import is not
			// a consumable dependency.
		}
	}
	return vs, nil
}

// edge is one internal import occurrence lifted to Zone level.
// from is "" when the importing file belongs to no declared Zone.
type edge struct {
	from, to    rule.ZoneName
	path        string
	line        int
	importPath  string
	sourceZones []rule.ZoneName
}

// evaluateGraph judges layers, protected, independence, and acyclic
// Rules over the observed import graph.
func evaluateGraph(r rule.Rule, supplied Facts) ([]Evaluation, error) {
	if p, ok := r.Params().(rule.IndependenceParams); ok {
		return evaluateIndependence(r, p, supplied)
	}
	edges, err := supplied.zoneEdges()
	if err != nil {
		return nil, fmt.Errorf("rule %s: %w", r.ID(), err)
	}
	switch p := r.Params().(type) {
	case rule.LayersParams:
		return evaluateLayers(r, p, edges)
	case rule.ProtectedParams:
		return evaluateProtected(r, p, edges)
	case rule.AcyclicParams:
		if r.BuiltIn() {
			edges = betweenSeparateCodes(edges, supplied)
		}
		return evaluateAcyclic(r, p, edges, supplied)
	}
	return nil, fmt.Errorf("rule %s: graph rule with %T params", r.ID(), r.Params())
}

// betweenSeparateCodes keeps the edges that are dependencies between two
// codes, for the built-in acyclic rule. It drops the edges between a
// Zone and a Zone nested inside it, where one Zone's observed files are
// all files of the other, since an import across that pair is movement
// within one code; and it drops the imports of a Go external test
// package (package name ending in _test), which Go compiles as a
// separate package, so its imports are the test's and not the Zone's.
func betweenSeparateCodes(edges []edge, supplied Facts) []edge {
	out := make([]edge, 0, len(edges))
	for _, e := range edges {
		if e.from != "" && supplied.sameCodeZones(e.from, e.to) {
			continue
		}
		if supplied.externalTestPackage(e.path) {
			continue
		}
		out = append(out, e)
	}
	return out
}

// evaluateLayers judges each layered Zone: it may import same or
// lower layers, never higher.
func evaluateLayers(r rule.Rule, p rule.LayersParams, edges []edge) ([]Evaluation, error) {
	index := map[rule.ZoneName]int{}
	for i, name := range p.Layers {
		index[name] = i
	}
	var out []Evaluation
	for _, name := range p.Layers {
		if r.Scope().ExcludedZone(name) {
			subject, err := rule.ZoneSubject(name)
			if err != nil {
				return nil, fmt.Errorf("layers: %w", err)
			}
			e, err := simpleEvaluation(r, subject, OutcomeNotApplicable)
			if err != nil {
				return nil, err
			}
			out = append(out, e)
			continue
		}
		subject, err := rule.ZoneSubject(name)
		if err != nil {
			return nil, fmt.Errorf("layers: %w", err)
		}
		var vs []Violation
		for _, w := range edges {
			if w.from != name {
				continue
			}
			ti, higher := index[w.to]
			if !higher || ti >= index[name] {
				continue
			}
			v, err := newViolation(r, subject, w.path, w.line,
				fmt.Sprintf("Zone %q (layer %d) may not import higher layer %q (layer %d): import of %q",
					name, index[name], w.to, ti, w.importPath),
				"depend only on Zones at the same or a lower layer")
			if err != nil {
				return nil, err
			}
			vs = append(vs, v)
		}
		e, err := completeEvaluation(r, subject, vs)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

// evaluateProtected judges the protected Zone: an importer is
// allowed when any of its Zones is the protected Zone or on the
// allow-list. One Violation per import occurrence.
func evaluateProtected(r rule.Rule, p rule.ProtectedParams, edges []edge) ([]Evaluation, error) {
	subject, err := rule.ZoneSubject(p.Zone)
	if err != nil {
		return nil, fmt.Errorf("protected: %w", err)
	}
	if r.Scope().ExcludedZone(p.Zone) {
		e, err := simpleEvaluation(r, subject, OutcomeNotApplicable)
		if err != nil {
			return nil, err
		}
		return []Evaluation{e}, nil
	}
	allowed := append([]rule.ZoneName{p.Zone}, p.Allow...)
	type occurrence struct {
		path string
		line int
	}
	seen := map[occurrence]bool{}
	var vs []Violation
	for _, w := range edges {
		if w.to != p.Zone {
			continue
		}
		importers := w.sourceZones
		permitted := false
		for _, m := range importers {
			if zoneIn(allowed, m) {
				permitted = true
				break
			}
		}
		if permitted {
			continue
		}
		occ := occurrence{w.path, w.line}
		if seen[occ] {
			continue
		}
		seen[occ] = true
		source := fmt.Sprintf("Zone(s) %s", quotedZones(importers))
		if len(importers) == 0 {
			source = "code outside any declared Zone"
		}
		v, err := newViolation(r, subject, w.path, w.line,
			fmt.Sprintf("Zone %q is protected (importable only by %s); import of %q from %s",
				p.Zone, quotedZones(p.Allow), w.importPath, source),
			fmt.Sprintf("add the importer to the allow-list protecting %q or remove the import", p.Zone))
		if err != nil {
			return nil, err
		}
		vs = append(vs, v)
	}
	e, err := completeEvaluation(r, subject, vs)
	if err != nil {
		return nil, err
	}
	return []Evaluation{e}, nil
}

// evaluateIndependence judges sibling Folders: a file under member A
// may not import a target under member B. Zone-owned candidates are
// not members. Fewer than two members is vacuously satisfied.
func evaluateIndependence(r rule.Rule, p rule.IndependenceParams, supplied Facts) ([]Evaluation, error) {
	members, err := supplied.independentFolders(p)
	if err != nil {
		return nil, fmt.Errorf("rule %s: %w", r.ID(), err)
	}
	subject, err := independenceSubject(members, p)
	if err != nil {
		return nil, err
	}
	if len(members) < 2 {
		e, err := completeEvaluation(r, subject, nil)
		if err != nil {
			return nil, err
		}
		return []Evaluation{e}, nil
	}
	var vs []Violation
	for _, path := range supplied.paths() {
		from, ok := independenceFolderOf(path, members)
		if !ok {
			continue
		}
		for _, imp := range supplied.ImportsFor(path) {
			if imp.Class != ImportInternal {
				continue
			}
			target := imp.TargetFile
			if target == "" {
				target = imp.TargetDir
			}
			if target == "" {
				continue
			}
			to, ok := independenceFolderOf(target, members)
			if !ok || to == from {
				continue
			}
			v, err := newViolation(r, subject, path, imp.Line,
				fmt.Sprintf("Folder %q is independent of sibling Folder %q: import of %q", from, to, imp.Path),
				"remove the cross-folder import")
			if err != nil {
				return nil, err
			}
			vs = append(vs, v)
		}
	}
	e, err := completeEvaluation(r, subject, vs)
	if err != nil {
		return nil, err
	}
	return []Evaluation{e}, nil
}

func independenceFolderOf(path string, members []string) (string, bool) {
	matched := ""
	for _, m := range members {
		if path == m || strings.HasPrefix(path, m+"/") {
			if len(m) > len(matched) {
				matched = m
			}
		}
	}
	if matched == "" {
		return "", false
	}
	return matched, true
}

func independenceSubject(members []string, p rule.IndependenceParams) (rule.Subject, error) {
	id := p.Folders[0].String()
	if len(members) > 0 {
		id = members[0]
	}
	subject, err := rule.FolderSubject(id)
	if err != nil {
		return rule.Subject{}, fmt.Errorf("independence: %w", err)
	}
	return subject, nil
}

// evaluateAcyclic judges each Zone in scope: it participates in no
// dependency cycle. Every cycle member reports with its own witness
// edge inside the cycle.
func evaluateAcyclic(r rule.Rule, p rule.AcyclicParams, edges []edge, supplied Facts) ([]Evaluation, error) {
	scope := p.Zones
	if len(scope) == 0 {
		scope = supplied.zoneNames()
	}
	scope = sortedZones(scope)
	cycles := stronglyConnected(scope, edges)
	var out []Evaluation
	for _, name := range scope {
		subject, err := rule.ZoneSubject(name)
		if err != nil {
			return nil, fmt.Errorf("acyclic: %w", err)
		}
		if r.Scope().ExcludedZone(name) {
			e, err := simpleEvaluation(r, subject, OutcomeNotApplicable)
			if err != nil {
				return nil, err
			}
			out = append(out, e)
			continue
		}
		cycle := cycles[name]
		var vs []Violation
		if len(cycle) > 1 {
			witness, found := witnessEdge(name, cycle, edges)
			if !found {
				return nil, fmt.Errorf("rule %s: zone %q in cycle without witness edge", r.ID(), name)
			}
			v, err := newViolation(r, subject, witness.path, witness.line,
				fmt.Sprintf("Zone %q participates in a dependency cycle: %s",
					name, joinZones(cycle, " -> ")),
				"break the cycle by inverting or removing one dependency")
			if err != nil {
				return nil, err
			}
			vs = append(vs, v)
		}
		e, err := completeEvaluation(r, subject, vs)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

// stronglyConnected maps each Zone to its strongly connected
// component of size > 1 (its cycle), using Tarjan's algorithm with
// iteration order fixed by sorted names.
func stronglyConnected(nodes []rule.ZoneName, edges []edge) map[rule.ZoneName][]rule.ZoneName {
	inScope := map[rule.ZoneName]bool{}
	for _, n := range nodes {
		inScope[n] = true
	}
	adj := map[rule.ZoneName][]rule.ZoneName{}
	seen := map[[2]rule.ZoneName]bool{}
	for _, w := range edges {
		if w.from == "" || w.from == w.to || !inScope[w.from] || !inScope[w.to] {
			continue
		}
		key := [2]rule.ZoneName{w.from, w.to}
		if !seen[key] {
			seen[key] = true
			adj[w.from] = append(adj[w.from], w.to)
		}
	}

	index := map[rule.ZoneName]int{}
	low := map[rule.ZoneName]int{}
	onStack := map[rule.ZoneName]bool{}
	var stack []rule.ZoneName
	next := 0
	cycles := map[rule.ZoneName][]rule.ZoneName{}
	var strongconnect func(v rule.ZoneName)
	strongconnect = func(v rule.ZoneName) {
		index[v] = next
		low[v] = next
		next++
		stack = append(stack, v)
		onStack[v] = true
		for _, w := range adj[v] {
			if _, ok := index[w]; !ok {
				strongconnect(w)
				if low[w] < low[v] {
					low[v] = low[w]
				}
			} else if onStack[w] && index[w] < low[v] {
				low[v] = index[w]
			}
		}
		if low[v] == index[v] {
			var scc []rule.ZoneName
			for {
				n := len(stack) - 1
				w := stack[n]
				stack = stack[:n]
				onStack[w] = false
				scc = append(scc, w)
				if w == v {
					break
				}
			}
			if len(scc) > 1 {
				scc = sortedZones(scc)
				for _, m := range scc {
					cycles[m] = scc
				}
			}
		}
	}
	for _, v := range sortedZones(nodes) {
		if _, ok := index[v]; !ok {
			strongconnect(v)
		}
	}
	return cycles
}

// witnessEdge finds the first import occurrence from the Zone to
// another cycle member.
func witnessEdge(from rule.ZoneName, cycle []rule.ZoneName, edges []edge) (edge, bool) {
	for _, w := range edges {
		if w.from == from && zoneIn(cycle, w.to) {
			return w, true
		}
	}
	return edge{}, false
}

func zoneIn(list []rule.ZoneName, m rule.ZoneName) bool {
	for _, v := range list {
		if v == m {
			return true
		}
	}
	return false
}

func quotedZones(names []rule.ZoneName) string {
	if len(names) == 0 {
		return "[]"
	}
	sorted := append([]rule.ZoneName(nil), names...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	quoted := make([]string, len(sorted))
	for i, n := range sorted {
		quoted[i] = fmt.Sprintf("%q", string(n))
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

func joinZones(names []rule.ZoneName, sep string) string {
	parts := make([]string, len(names))
	for i, n := range names {
		parts[i] = string(n)
	}
	return strings.Join(parts, sep)
}
