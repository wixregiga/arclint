package engine

import (
	"fmt"
	"sort"
	"strings"

	"github.com/wixregiga/arclint/internal/lang/golang"
	"github.com/wixregiga/arclint/internal/report"
)

// checkConsumes evaluates every per-module consumes clause against the
// import analysis. Blame is always the consumer: the violation anchors at
// the importing file and line.
func checkConsumes(ctx *Context) []report.Violation {
	if ctx.Go == nil {
		return nil
	}
	var vs []report.Violation
	for _, f := range ctx.Tree.Files {
		fa := ctx.Go.Files[f.Path]
		if fa == nil {
			continue
		}
		for _, m := range ctx.FileModules[f.Path] {
			c := ctx.RS.Contracts[m].Consumes
			if c == nil {
				continue
			}
			sev := report.Severity(severityOf(c.Severity))
			for _, imp := range fa.Imports {
				switch imp.Class {
				case golang.ClassStdlib:
					if c.Stdlib == "forbid" {
						vs = append(vs, report.Violation{
							RuleID: m + ".consumes.stdlib", Contract: report.ContractConsumes,
							Blame: report.BlameConsumer, Severity: sev,
							Path: f.Path, Line: report.IntPtr(imp.Line),
							Message: fmt.Sprintf("stdlib import %q forbidden by contract %q", imp.Path, m),
							FixHint: fmt.Sprintf("remove the import or set contracts.%s.consumes.stdlib: allow", m),
						})
					}
				case golang.ClassExternal:
					if c.External == "forbid" {
						vs = append(vs, report.Violation{
							RuleID: m + ".consumes.external", Contract: report.ContractConsumes,
							Blame: report.BlameConsumer, Severity: sev,
							Path: f.Path, Line: report.IntPtr(imp.Line),
							Message: fmt.Sprintf("external import %q forbidden by contract %q", imp.Path, m),
							FixHint: fmt.Sprintf("remove the import or set contracts.%s.consumes.external: allow", m),
						})
					}
				case golang.ClassInternal:
					if c.Internal == nil {
						continue
					}
					targets := ctx.DirModules[imp.TargetDir]
					if contains(targets, m) {
						continue // own module, never a violation
					}
					if len(c.Internal.Deny) > 0 && intersects(targets, c.Internal.Deny) {
						vs = append(vs, report.Violation{
							RuleID: m + ".consumes.internal", Contract: report.ContractConsumes,
							Blame: report.BlameConsumer, Severity: sev,
							Path: f.Path, Line: report.IntPtr(imp.Line),
							Message: fmt.Sprintf("import %q resolves to module(s) %s, denied by contract %q",
								imp.Path, quoteList(setIntersect(targets, c.Internal.Deny)), m),
							FixHint: fmt.Sprintf("remove the import or drop it from contracts.%s.consumes.internal.deny", m),
						})
						continue
					}
					if c.Internal.Restricted {
						allowed := append([]string{m}, c.Internal.Allow...)
						if !intersects(targets, allowed) {
							var msg string
							switch {
							case imp.TargetDir == "":
								msg = fmt.Sprintf("import %q is internal (replace-to-local outside the tree) and not allowed by contract %q", imp.Path, m)
							case len(targets) == 0:
								msg = fmt.Sprintf("import %q resolves to internal code outside any declared module, not allowed by contract %q", imp.Path, m)
							default:
								msg = fmt.Sprintf("import %q resolves to module(s) %s, not in the allow-list of contract %q",
									imp.Path, quoteList(targets), m)
							}
							vs = append(vs, report.Violation{
								RuleID: m + ".consumes.internal", Contract: report.ContractConsumes,
								Blame: report.BlameConsumer, Severity: sev,
								Path: f.Path, Line: report.IntPtr(imp.Line),
								Message: msg,
								FixHint: fmt.Sprintf("add the target module to contracts.%s.consumes.internal or remove the import", m),
							})
						}
					}
				}
			}
		}
	}
	return vs
}

// checkUnknownImports applies the repo-wide unknown-import policy
// (default warn): imports that are neither stdlib, internal, nor declared
// in go.mod.
func checkUnknownImports(ctx *Context) []report.Violation {
	if ctx.Go == nil {
		return nil
	}
	policy := ctx.RS.Scan.UnknownImports
	if policy == "" {
		policy = "warn"
	}
	if policy == "ignore" {
		return nil
	}
	var vs []report.Violation
	for _, f := range ctx.Tree.Files {
		fa := ctx.Go.Files[f.Path]
		if fa == nil {
			continue
		}
		for _, imp := range fa.Imports {
			if imp.Class != golang.ClassUnknown {
				continue
			}
			msg := fmt.Sprintf("import %q is neither stdlib, internal, nor resolvable via go.mod require", imp.Path)
			if policy == "error" {
				vs = append(vs, report.Violation{
					RuleID: "scan.unknown-imports", Contract: report.ContractConsumes,
					Blame: report.BlameConsumer, Severity: report.SeverityError,
					Path: f.Path, Line: report.IntPtr(imp.Line),
					Message: msg,
					FixHint: "declare the dependency in go.mod or set scan.unknown_imports to warn/ignore",
				})
			} else {
				ctx.Warn("%s:%d: %s (unknown_imports: warn)", f.Path, imp.Line, msg)
			}
		}
	}
	return vs
}

// edgeWitness is one internal import occurrence lifted to module level.
// from is "" when the importing file belongs to no declared module.
type edgeWitness struct {
	from, to   string
	file       string
	line       int
	importPath string
}

// moduleEdges lifts the file-level import graph to declared modules,
// preserving file order for deterministic reporting.
func moduleEdges(ctx *Context) []edgeWitness {
	if ctx.Go == nil {
		return nil
	}
	var ws []edgeWitness
	for _, f := range ctx.Tree.Files {
		fa := ctx.Go.Files[f.Path]
		if fa == nil {
			continue
		}
		froms := ctx.FileModules[f.Path]
		for _, imp := range fa.Imports {
			if imp.Class != golang.ClassInternal || imp.TargetDir == "" {
				continue
			}
			targets := ctx.DirModules[imp.TargetDir]
			for _, to := range targets {
				if len(froms) == 0 {
					ws = append(ws, edgeWitness{from: "", to: to, file: f.Path, line: imp.Line, importPath: imp.Path})
					continue
				}
				for _, from := range froms {
					if from == to {
						continue
					}
					ws = append(ws, edgeWitness{from: from, to: to, file: f.Path, line: imp.Line, importPath: imp.Path})
				}
			}
		}
	}
	return ws
}

// checkGraphRules evaluates the graph-wide consumes clauses: layers,
// forbidden, independence, protected, acyclic.
func checkGraphRules(ctx *Context) []report.Violation {
	if len(ctx.RS.Dependencies) == 0 {
		return nil
	}
	witnesses := moduleEdges(ctx)
	var vs []report.Violation
	for i, rule := range ctx.RS.Dependencies {
		id := rule.ID
		if id == "" {
			id = fmt.Sprintf("dependencies.%s[%d]", rule.Kind, i)
		}
		sev := report.Severity(severityOf(rule.Severity))
		switch rule.Kind {
		case "layers":
			idx := map[string]int{}
			for n, name := range rule.Layers {
				idx[name] = n
			}
			for _, w := range witnesses {
				if w.from == "" {
					continue
				}
				fi, okF := idx[w.from]
				ti, okT := idx[w.to]
				if okF && okT && ti < fi {
					vs = append(vs, report.Violation{
						RuleID: id, Contract: report.ContractConsumes,
						Blame: report.BlameConsumer, Severity: sev,
						Path: w.file, Line: report.IntPtr(w.line),
						Message: fmt.Sprintf("module %q (layer %d) may not import higher layer %q (layer %d): import of %q",
							w.from, fi, w.to, ti, w.importPath),
						FixHint: "depend only on modules at the same or a lower layer",
					})
				}
			}
		case "forbidden":
			for _, w := range witnesses {
				if w.from != "" && contains(rule.From, w.from) && contains(rule.To, w.to) {
					vs = append(vs, report.Violation{
						RuleID: id, Contract: report.ContractConsumes,
						Blame: report.BlameConsumer, Severity: sev,
						Path: w.file, Line: report.IntPtr(w.line),
						Message: fmt.Sprintf("forbidden dependency: module %q imports %q (import of %q)",
							w.from, w.to, w.importPath),
						FixHint: "remove the dependency or drop it from the forbidden rule",
					})
				}
			}
		case "independence":
			for _, w := range witnesses {
				if w.from != "" && w.from != w.to && contains(rule.Modules, w.from) && contains(rule.Modules, w.to) {
					vs = append(vs, report.Violation{
						RuleID: id, Contract: report.ContractConsumes,
						Blame: report.BlameConsumer, Severity: sev,
						Path: w.file, Line: report.IntPtr(w.line),
						Message: fmt.Sprintf("modules %q and %q are declared independent: import of %q",
							w.from, w.to, w.importPath),
						FixHint: "remove the dependency or drop a module from the independence set",
					})
				}
			}
		case "protected":
			allowed := append([]string{rule.Module}, rule.Allow...)
			// Protection is importer-centric: a file is an allowed
			// importer when ANY of its modules is in the allow set, so a
			// file in both an allowed module and an overlapping umbrella
			// module does not violate. One violation per import
			// occurrence, not per overlapping membership.
			type occurrence struct {
				file string
				line int
			}
			seenOcc := map[occurrence]bool{}
			for _, w := range witnesses {
				if w.to != rule.Module {
					continue
				}
				importerModules := ctx.FileModules[w.file]
				if intersects(importerModules, allowed) {
					continue
				}
				occ := occurrence{w.file, w.line}
				if seenOcc[occ] {
					continue
				}
				seenOcc[occ] = true
				src := fmt.Sprintf("modules %s", quoteList(importerModules))
				if len(importerModules) == 0 {
					src = "code outside any declared module"
				}
				vs = append(vs, report.Violation{
					RuleID: id, Contract: report.ContractConsumes,
					Blame: report.BlameConsumer, Severity: sev,
					Path: w.file, Line: report.IntPtr(w.line),
					Message: fmt.Sprintf("module %q is protected (importable only by %s); import of %q from %s",
						rule.Module, quoteList(rule.Allow), w.importPath, src),
					FixHint: fmt.Sprintf("add the importer to the allow list of the protected rule for %q or remove the import", rule.Module),
				})
			}
		case "acyclic":
			nodes := rule.Modules
			if len(nodes) == 0 {
				nodes = ctx.ModuleNames
			}
			vs = append(vs, checkAcyclic(id, sev, nodes, witnesses)...)
		}
	}
	return vs
}

// checkAcyclic finds strongly connected components of size > 1 among the
// given modules and reports one violation per cycle.
func checkAcyclic(id string, sev report.Severity, nodes []string, witnesses []edgeWitness) []report.Violation {
	inSet := map[string]bool{}
	for _, n := range nodes {
		inSet[n] = true
	}
	adj := map[string][]string{}
	seen := map[[2]string]bool{}
	for _, w := range witnesses {
		if w.from == "" || w.from == w.to || !inSet[w.from] || !inSet[w.to] {
			continue
		}
		key := [2]string{w.from, w.to}
		if !seen[key] {
			seen[key] = true
			adj[w.from] = append(adj[w.from], w.to)
		}
	}

	// Tarjan's algorithm, iterative order fixed by sorted node names.
	sorted := append([]string(nil), nodes...)
	sort.Strings(sorted)
	index := map[string]int{}
	low := map[string]int{}
	onStack := map[string]bool{}
	var stack []string
	next := 0
	var sccs [][]string
	var strongconnect func(v string)
	strongconnect = func(v string) {
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
			} else if onStack[w] {
				if index[w] < low[v] {
					low[v] = index[w]
				}
			}
		}
		if low[v] == index[v] {
			var scc []string
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
				sort.Strings(scc)
				sccs = append(sccs, scc)
			}
		}
	}
	for _, v := range sorted {
		if _, ok := index[v]; !ok {
			strongconnect(v)
		}
	}

	var vs []report.Violation
	for _, scc := range sccs {
		anchorFile, anchorLine := "", 0
		for _, w := range witnesses {
			if w.from != "" && contains(scc, w.from) && contains(scc, w.to) {
				anchorFile, anchorLine = w.file, w.line
				break
			}
		}
		v := report.Violation{
			RuleID: id, Contract: report.ContractConsumes,
			Blame: report.BlameConsumer, Severity: sev,
			Path:    anchorFile,
			Message: fmt.Sprintf("dependency cycle among modules: %s", strings.Join(scc, " -> ")),
			FixHint: "break the cycle by inverting or removing one dependency",
		}
		if anchorLine > 0 {
			v.Line = report.IntPtr(anchorLine)
		}
		vs = append(vs, v)
	}
	return vs
}

func quoteList(names []string) string {
	if len(names) == 0 {
		return "[]"
	}
	quoted := make([]string, len(names))
	for i, n := range names {
		quoted[i] = fmt.Sprintf("%q", n)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

func setIntersect(a, b []string) []string {
	var out []string
	for _, v := range a {
		if contains(b, v) {
			out = append(out, v)
		}
	}
	return out
}
