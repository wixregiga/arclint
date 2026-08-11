package engine

import (
	"bytes"
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/wixregiga/arclint/internal/config"
	"github.com/wixregiga/arclint/internal/exprenv"
	"github.com/wixregiga/arclint/internal/report"
	"github.com/wixregiga/arclint/internal/tree"
)

// namedCaseRules is the ls-lint-style closed naming vocabulary, applied to
// the file stem (base name without final extension).
var namedCaseRules = map[string]*regexp.Regexp{
	"kebab-case": regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`),
	"snake_case": regexp.MustCompile(`^[a-z0-9]+(_[a-z0-9]+)*$`),
	"camelCase":  regexp.MustCompile(`^[a-z][a-z0-9]*([A-Z][a-z0-9]*)*$`),
	"PascalCase": regexp.MustCompile(`^([A-Z][a-z0-9]*)+$`),
}

// checkInvariants evaluates naming, structure, content, and expr rules.
func checkInvariants(ctx *Context) []report.Violation {
	var vs []report.Violation
	for _, m := range ctx.ModuleNames {
		for i, rule := range ctx.RS.Contracts[m].Invariants {
			id := rule.ID
			if id == "" {
				id = fmt.Sprintf("%s.invariants.%s[%d]", m, rule.Kind, i)
			}
			sev := report.Severity(severityOf(rule.Severity))
			switch rule.Kind {
			case "naming":
				vs = append(vs, checkNaming(ctx, m, id, sev, rule)...)
			case "structure":
				vs = append(vs, checkStructure(ctx, m, id, sev, rule)...)
			case "content":
				vs = append(vs, checkContent(ctx, m, id, sev, rule)...)
			case "expr":
				vs = append(vs, checkExpr(ctx, m, id, sev, rule)...)
			}
		}
	}
	return vs
}

// moduleFilesFiltered returns the module's files, optionally narrowed by
// the rule's Files glob.
func moduleFilesFiltered(ctx *Context, module, filesGlob string) []*tree.File {
	files := ctx.ModuleFiles[module]
	if filesGlob == "" {
		return files
	}
	var out []*tree.File
	for _, f := range files {
		if ok, _ := doublestar.Match(filesGlob, f.Path); ok {
			out = append(out, f)
		}
	}
	return out
}

func checkNaming(ctx *Context, module, id string, sev report.Severity, rule config.InvariantRule) []report.Violation {
	alternatives := strings.Split(rule.Case, "|")
	type alt struct {
		label string
		re    *regexp.Regexp
	}
	var alts []alt
	for _, a := range alternatives {
		a = strings.TrimSpace(a)
		if re, ok := namedCaseRules[a]; ok {
			alts = append(alts, alt{a, re})
			continue
		}
		if pat, ok := strings.CutPrefix(a, "regex:"); ok {
			re, err := regexp.Compile("^(?:" + pat + ")$")
			if err != nil {
				ctx.Warn("rule %s: invalid regex case %q: %v", id, a, err)
				continue
			}
			alts = append(alts, alt{a, re})
		} else {
			ctx.Warn("rule %s: unknown case %q", id, a)
		}
	}
	if len(alts) == 0 {
		return nil
	}
	var vs []report.Violation
	for _, f := range moduleFilesFiltered(ctx, module, rule.Files) {
		ok := false
		var labels []string
		for _, a := range alts {
			labels = append(labels, a.label)
			if a.re.MatchString(f.Stem()) {
				ok = true
				break
			}
		}
		if !ok {
			vs = append(vs, report.Violation{
				RuleID: id, Contract: report.ContractInvariant,
				Blame: report.BlameProvider, Severity: sev,
				Path:    f.Path,
				Message: fmt.Sprintf("file name %q violates naming rule %s", f.Name(), strings.Join(labels, " | ")),
				FixHint: fmt.Sprintf("rename the file so its stem matches %s", strings.Join(labels, " | ")),
			})
		}
	}
	return vs
}

// staticPrefix returns the longest glob-free directory prefix of a pattern,
// used to anchor structure violations at a real path.
func staticPrefix(pattern string) string {
	segs := strings.Split(pattern, "/")
	var keep []string
	for _, s := range segs {
		if strings.ContainsAny(s, "*?[{") {
			break
		}
		keep = append(keep, s)
	}
	if len(keep) == 0 {
		return "."
	}
	if len(keep) == len(segs) {
		return pattern
	}
	return path.Join(keep...)
}

func checkStructure(ctx *Context, module, id string, sev report.Severity, rule config.InvariantRule) []report.Violation {
	var vs []report.Violation
	for _, req := range rule.Require {
		found := false
		for _, f := range ctx.ModuleFiles[module] {
			if ok, _ := doublestar.Match(req, f.Path); ok {
				found = true
				break
			}
		}
		if !found {
			vs = append(vs, report.Violation{
				RuleID: id, Contract: report.ContractInvariant,
				Blame: report.BlameProvider, Severity: sev,
				Path:    staticPrefix(req),
				Message: fmt.Sprintf("module %q is missing a required file matching %q", module, req),
				FixHint: fmt.Sprintf("create a file matching %q", req),
			})
		}
	}
	for _, forb := range rule.Forbid {
		for _, f := range ctx.ModuleFiles[module] {
			if ok, _ := doublestar.Match(forb, f.Path); ok {
				vs = append(vs, report.Violation{
					RuleID: id, Contract: report.ContractInvariant,
					Blame: report.BlameProvider, Severity: sev,
					Path:    f.Path,
					Message: fmt.Sprintf("path forbidden by structure rule %q of module %q", forb, module),
					FixHint: "remove or relocate the file",
				})
			}
		}
	}
	return vs
}

func checkContent(ctx *Context, module, id string, sev report.Severity, rule config.InvariantRule) []report.Violation {
	var vs []report.Violation
	files := moduleFilesFiltered(ctx, module, rule.Files)
	for _, pat := range rule.Must {
		re, err := regexp.Compile(pat)
		if err != nil {
			ctx.Warn("rule %s: invalid must regex %q: %v", id, pat, err)
			continue
		}
		for _, f := range files {
			if !re.Match(ctx.Content(f)) {
				vs = append(vs, report.Violation{
					RuleID: id, Contract: report.ContractInvariant,
					Blame: report.BlameProvider, Severity: sev,
					Path:    f.Path,
					Message: fmt.Sprintf("file lacks required content matching /%s/", pat),
					FixHint: fmt.Sprintf("add content matching /%s/", pat),
				})
			}
		}
	}
	for _, pat := range rule.MustNot {
		re, err := regexp.Compile(pat)
		if err != nil {
			ctx.Warn("rule %s: invalid must_not regex %q: %v", id, pat, err)
			continue
		}
		for _, f := range files {
			content := ctx.Content(f)
			for _, loc := range re.FindAllIndex(content, -1) {
				line := 1 + bytes.Count(content[:loc[0]], []byte("\n"))
				vs = append(vs, report.Violation{
					RuleID: id, Contract: report.ContractInvariant,
					Blame: report.BlameProvider, Severity: sev,
					Path: f.Path, Line: report.IntPtr(line),
					Message: fmt.Sprintf("forbidden content matching /%s/", pat),
					FixHint: fmt.Sprintf("remove the content matching /%s/", pat),
				})
			}
		}
	}
	return vs
}

func checkExpr(ctx *Context, module, id string, sev report.Severity, rule config.InvariantRule) []report.Violation {
	prog, err := exprenv.Compile(rule.Assert)
	if err != nil {
		ctx.Warn("rule %s: %v", id, err)
		return nil
	}
	var vs []report.Violation
	for _, f := range moduleFilesFiltered(ctx, module, rule.Files) {
		content := ctx.Content(f)
		lines := 0
		if len(content) > 0 {
			lines = bytes.Count(content, []byte("\n"))
			if content[len(content)-1] != '\n' {
				lines++
			}
		}
		var imports []string
		if ctx.Go != nil {
			if fa := ctx.Go.Files[f.Path]; fa != nil {
				for _, imp := range fa.Imports {
					imports = append(imports, imp.Path)
				}
			}
		}
		env := exprenv.File{
			Path: f.Path, Name: f.Name(), Stem: f.Stem(), Ext: f.Ext(), Dir: f.Dir(),
			Module: module, Lines: lines, Size: int(f.Size), Imports: imports,
		}
		ok, err := exprenv.Run(prog, env)
		if err != nil {
			ctx.Warn("rule %s: %s: %v", id, f.Path, err)
			continue
		}
		if !ok {
			msg := rule.Message
			if msg == "" {
				msg = fmt.Sprintf("expr assertion failed: %s", rule.Assert)
			}
			vs = append(vs, report.Violation{
				RuleID: id, Contract: report.ContractInvariant,
				Blame: report.BlameProvider, Severity: sev,
				Path:    f.Path,
				Message: msg,
				FixHint: fmt.Sprintf("make the file satisfy: %s", rule.Assert),
			})
		}
	}
	return vs
}
