package conformance

import (
	"fmt"
	"sort"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/rule"
)

// evaluateConsumes judges every code file of the Rule's Modules
// against the import policy. Files in languages without produced facts
// evaluate unsupported; analysis failures evaluate failed.
func evaluateConsumes(r rule.Rule, mem membership, obs Observations) ([]Evaluation, error) {
	p, ok := r.Params().(rule.ConsumesParams)
	if !ok {
		return nil, fmt.Errorf("rule %s: consumes rule with %T params", r.ID(), r.Params())
	}
	var out []Evaluation
	for _, name := range sortedModules(r.Applicability().Modules()) {
		if r.Applicability().ExcludedModule(name) {
			subject, err := rule.ModuleSubject(name)
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
		selected, excluded := partitionModuleFiles(r, name, mem)
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
			facts, have := obs.FactsFor(f)
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
			vs, err := consumesViolations(r, p, name, subject, f, facts.Imports, mem)
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

func consumesViolations(r rule.Rule, p rule.ConsumesParams, self rule.ModuleName,
	subject rule.Subject, path string, imports []Import, mem membership,
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
					fmt.Sprintf("standard-library import %q is forbidden for Module %q", imp.Path, self),
					"remove the import or allow standard-library imports for the Module"); err != nil {
					return nil, err
				}
			}
		case ImportExternal:
			if p.External.Forbids() {
				if err := add(imp.Line,
					fmt.Sprintf("external import %q is forbidden for Module %q", imp.Path, self),
					"remove the import or allow external imports for the Module"); err != nil {
					return nil, err
				}
			}
		case ImportInternal:
			if p.Internal == nil {
				continue
			}
			targets := mem.targetModules(imp)
			if moduleIn(targets, self) {
				continue // the Module itself, never a violation
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
				message = fmt.Sprintf("import %q is internal but does not resolve into the repository, and Module %q restricts internal imports", imp.Path, self)
			case len(targets) == 0:
				message = fmt.Sprintf("import %q resolves to code outside any declared Module, not allowed for Module %q", imp.Path, self)
			default:
				message = fmt.Sprintf("import %q resolves to Module(s) %s, not in the allow-list of Module %q",
					imp.Path, quotedModules(targets), self)
			}
			if err := add(imp.Line, message,
				"add the target Module to the allow-list or remove the import"); err != nil {
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

// edge is one internal import occurrence lifted to Module level.
// from is "" when the importing file belongs to no declared Module.
type edge struct {
	from, to   rule.ModuleName
	path       string
	line       int
	importPath string
}

// moduleEdges lifts the file-level import graph to declared Modules,
// preserving file order for deterministic reporting.
func moduleEdges(mem membership, obs Observations) []edge {
	var out []edge
	for _, f := range mem.files {
		facts, ok := obs.FactsFor(f)
		if !ok || !facts.Supports(rule.FactImports) {
			continue
		}
		froms := mem.fileModules[f]
		for _, imp := range facts.Imports {
			if imp.Class != ImportInternal {
				continue
			}
			for _, to := range mem.targetModules(imp) {
				if len(froms) == 0 {
					out = append(out, edge{from: "", to: to, path: f, line: imp.Line, importPath: imp.Path})
					continue
				}
				for _, from := range froms {
					if from == to {
						continue
					}
					out = append(out, edge{from: from, to: to, path: f, line: imp.Line, importPath: imp.Path})
				}
			}
		}
	}
	return out
}

// evaluateGraph judges layers, protected, independence, and acyclic
// Rules over the observed import graph.
func evaluateGraph(r rule.Rule, mem membership, obs Observations) ([]Evaluation, error) {
	edges := moduleEdges(mem, obs)
	switch p := r.Params().(type) {
	case rule.LayersParams:
		return evaluateLayers(r, p, edges)
	case rule.ProtectedParams:
		return evaluateProtected(r, p, edges, mem)
	case rule.IndependenceParams:
		return evaluateIndependence(r, p, mem, obs)
	case rule.AcyclicParams:
		return evaluateAcyclic(r, p, edges, mem)
	}
	return nil, fmt.Errorf("rule %s: graph rule with %T params", r.ID(), r.Params())
}

// evaluateLayers judges each layered Module: it may import same or
// lower layers, never higher.
func evaluateLayers(r rule.Rule, p rule.LayersParams, edges []edge) ([]Evaluation, error) {
	index := map[rule.ModuleName]int{}
	for i, name := range p.Layers {
		index[name] = i
	}
	var out []Evaluation
	for _, name := range p.Layers {
		if r.Applicability().ExcludedModule(name) {
			subject, err := rule.ModuleSubject(name)
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
		subject, err := rule.ModuleSubject(name)
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
				fmt.Sprintf("Module %q (layer %d) may not import higher layer %q (layer %d): import of %q",
					name, index[name], w.to, ti, w.importPath),
				"depend only on Modules at the same or a lower layer")
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

// evaluateProtected judges the protected Module: an importer is
// allowed when any of its Modules is the protected Module or on the
// allow-list. One Violation per import occurrence.
func evaluateProtected(r rule.Rule, p rule.ProtectedParams, edges []edge, mem membership) ([]Evaluation, error) {
	subject, err := rule.ModuleSubject(p.Module)
	if err != nil {
		return nil, fmt.Errorf("protected: %w", err)
	}
	if r.Applicability().ExcludedModule(p.Module) {
		e, err := simpleEvaluation(r, subject, OutcomeNotApplicable)
		if err != nil {
			return nil, err
		}
		return []Evaluation{e}, nil
	}
	allowed := append([]rule.ModuleName{p.Module}, p.Allow...)
	type occurrence struct {
		path string
		line int
	}
	seen := map[occurrence]bool{}
	var vs []Violation
	for _, w := range edges {
		if w.to != p.Module {
			continue
		}
		importers := mem.fileModules[w.path]
		permitted := false
		for _, m := range importers {
			if moduleIn(allowed, m) {
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
		source := fmt.Sprintf("Module(s) %s", quotedModules(importers))
		if len(importers) == 0 {
			source = "code outside any declared Module"
		}
		v, err := newViolation(r, subject, w.path, w.line,
			fmt.Sprintf("Module %q is protected (importable only by %s); import of %q from %s",
				p.Module, quotedModules(p.Allow), w.importPath, source),
			fmt.Sprintf("add the importer to the allow-list protecting %q or remove the import", p.Module))
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
// may not import a target under member B. Module-owned candidates are
// not members. Fewer than two members is vacuously satisfied.
func evaluateIndependence(r rule.Rule, p rule.IndependenceParams, mem membership, obs Observations) ([]Evaluation, error) {
	members := independenceMembers(p, mem)
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
	for _, path := range mem.files {
		from, ok := independenceFolderOf(path, members)
		if !ok {
			continue
		}
		facts, ok := obs.FactsFor(path)
		if !ok {
			continue
		}
		for _, imp := range facts.Imports {
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

func independenceMembers(p rule.IndependenceParams, mem membership) []string {
	seen := map[string]bool{}
	var out []string
	for _, g := range p.Folders {
		n := len(strings.Split(g.String(), "/"))
		for _, path := range mem.files {
			parts := strings.Split(path, "/")
			if len(parts) <= n {
				continue
			}
			candidate := strings.Join(parts[:n], "/")
			if seen[candidate] || !g.Match(candidate) {
				continue
			}
			if independenceModuleOwns(mem, candidate) {
				continue
			}
			seen[candidate] = true
			out = append(out, candidate)
		}
	}
	sort.Strings(out)
	return out
}

func independenceModuleOwns(mem membership, folder string) bool {
	for _, name := range mem.names {
		if mem.modules[name].Contains(folder) {
			return true
		}
	}
	return false
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

// evaluateAcyclic judges each Module in scope: it participates in no
// dependency cycle. Every cycle member reports with its own witness
// edge inside the cycle.
func evaluateAcyclic(r rule.Rule, p rule.AcyclicParams, edges []edge, mem membership) ([]Evaluation, error) {
	scope := p.Modules
	if len(scope) == 0 {
		scope = mem.names
	}
	scope = sortedModules(scope)
	cycles := stronglyConnected(scope, edges)
	var out []Evaluation
	for _, name := range scope {
		subject, err := rule.ModuleSubject(name)
		if err != nil {
			return nil, fmt.Errorf("acyclic: %w", err)
		}
		if r.Applicability().ExcludedModule(name) {
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
				return nil, fmt.Errorf("rule %s: module %q in cycle without witness edge", r.ID(), name)
			}
			v, err := newViolation(r, subject, witness.path, witness.line,
				fmt.Sprintf("Module %q participates in a dependency cycle: %s",
					name, joinModules(cycle, " -> ")),
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

// stronglyConnected maps each Module to its strongly connected
// component of size > 1 (its cycle), using Tarjan's algorithm with
// iteration order fixed by sorted names.
func stronglyConnected(nodes []rule.ModuleName, edges []edge) map[rule.ModuleName][]rule.ModuleName {
	inScope := map[rule.ModuleName]bool{}
	for _, n := range nodes {
		inScope[n] = true
	}
	adj := map[rule.ModuleName][]rule.ModuleName{}
	seen := map[[2]rule.ModuleName]bool{}
	for _, w := range edges {
		if w.from == "" || w.from == w.to || !inScope[w.from] || !inScope[w.to] {
			continue
		}
		key := [2]rule.ModuleName{w.from, w.to}
		if !seen[key] {
			seen[key] = true
			adj[w.from] = append(adj[w.from], w.to)
		}
	}

	index := map[rule.ModuleName]int{}
	low := map[rule.ModuleName]int{}
	onStack := map[rule.ModuleName]bool{}
	var stack []rule.ModuleName
	next := 0
	cycles := map[rule.ModuleName][]rule.ModuleName{}
	var strongconnect func(v rule.ModuleName)
	strongconnect = func(v rule.ModuleName) {
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
			var scc []rule.ModuleName
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
				scc = sortedModules(scc)
				for _, m := range scc {
					cycles[m] = scc
				}
			}
		}
	}
	for _, v := range sortedModules(nodes) {
		if _, ok := index[v]; !ok {
			strongconnect(v)
		}
	}
	return cycles
}

// witnessEdge finds the first import occurrence from the Module to
// another cycle member.
func witnessEdge(from rule.ModuleName, cycle []rule.ModuleName, edges []edge) (edge, bool) {
	for _, w := range edges {
		if w.from == from && moduleIn(cycle, w.to) {
			return w, true
		}
	}
	return edge{}, false
}

func moduleIn(list []rule.ModuleName, m rule.ModuleName) bool {
	for _, v := range list {
		if v == m {
			return true
		}
	}
	return false
}

func quotedModules(names []rule.ModuleName) string {
	if len(names) == 0 {
		return "[]"
	}
	sorted := append([]rule.ModuleName(nil), names...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	quoted := make([]string, len(sorted))
	for i, n := range sorted {
		quoted[i] = fmt.Sprintf("%q", string(n))
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

func joinModules(names []rule.ModuleName, sep string) string {
	parts := make([]string, len(names))
	for i, n := range names {
		parts[i] = string(n)
	}
	return strings.Join(parts, sep)
}
