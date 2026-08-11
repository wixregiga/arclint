package engine

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/wixregiga/arclint/internal/config"
	"github.com/wixregiga/arclint/internal/report"
)

// checkProvides evaluates registration and correspondence obligations.
// Blame is always the provider: the module that failed its promise.
func checkProvides(ctx *Context) []report.Violation {
	var vs []report.Violation
	for _, m := range ctx.ModuleNames {
		for i, rule := range ctx.RS.Contracts[m].Provides {
			id := rule.ID
			if id == "" {
				id = fmt.Sprintf("%s.provides.%s[%d]", m, rule.Kind, i)
			}
			sev := report.Severity(severityOf(rule.Severity))
			switch rule.Kind {
			case "registration":
				vs = append(vs, checkRegistration(ctx, m, id, sev, rule)...)
			case "correspondence":
				vs = append(vs, checkCorrespondence(ctx, m, id, sev, rule)...)
			}
		}
	}
	return vs
}

// namedCaptures extracts named groups of one match.
func namedCaptures(re *regexp.Regexp, match []string) map[string]string {
	caps := map[string]string{}
	for i, name := range re.SubexpNames() {
		if name != "" && i < len(match) {
			caps[name] = match[i]
		}
	}
	return caps
}

// expandValue substitutes {name} with the raw capture value.
func expandValue(template string, caps map[string]string) string {
	out := template
	for k, v := range caps {
		out = strings.ReplaceAll(out, "{"+k+"}", v)
	}
	return out
}

// expandPattern substitutes {name} with the regex-quoted capture value, so
// values can never inject pattern syntax.
func expandPattern(template string, caps map[string]string) string {
	out := template
	for k, v := range caps {
		out = strings.ReplaceAll(out, "{"+k+"}", regexp.QuoteMeta(v))
	}
	return out
}

// checkRegistration: every capture tuple of Each over the module's file
// paths must have a Match hit inside the files of the In module.
func checkRegistration(ctx *Context, module, id string, sev report.Severity, rule config.ProvidesRule) []report.Violation {
	eachRe, err := regexp.Compile(rule.Each)
	if err != nil {
		ctx.Warn("rule %s: invalid each regex: %v", id, err)
		return nil
	}
	type obligation struct {
		anchor  string // matched path prefix, trailing slash trimmed
		display string // first named capture value, or the anchor
		caps    map[string]string
	}
	seen := map[string]bool{}
	var obligations []obligation
	for _, f := range ctx.ModuleFiles[module] {
		match := eachRe.FindStringSubmatch(f.Path)
		if match == nil {
			continue
		}
		caps := namedCaptures(eachRe, match)
		key := match[0]
		for _, name := range eachRe.SubexpNames() {
			if name != "" {
				key += "\x00" + caps[name]
			}
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		ob := obligation{anchor: strings.TrimSuffix(match[0], "/"), caps: caps, display: strings.TrimSuffix(match[0], "/")}
		for _, name := range eachRe.SubexpNames() {
			if name != "" {
				ob.display = caps[name]
				break
			}
		}
		obligations = append(obligations, ob)
	}

	registryFiles := ctx.ModuleFiles[rule.InModule]
	var vs []report.Violation
	for _, ob := range obligations {
		pattern := expandPattern(rule.Match, ob.caps)
		re, err := regexp.Compile(pattern)
		if err != nil {
			ctx.Warn("rule %s: match template expands to invalid regex %q: %v", id, pattern, err)
			continue
		}
		satisfied := false
		for _, rf := range registryFiles {
			if re.Match(ctx.Content(rf)) {
				satisfied = true
				break
			}
		}
		if !satisfied {
			registryDesc := fmt.Sprintf("module %q", rule.InModule)
			if len(registryFiles) == 1 {
				registryDesc = registryFiles[0].Path
			}
			vs = append(vs, report.Violation{
				RuleID: id, Contract: report.ContractProvides,
				Blame: report.BlameProvider, Severity: sev,
				Path: ob.anchor,
				Message: fmt.Sprintf("%q is not registered: no content in %s matches /%s/",
					ob.display, registryDesc, pattern),
				FixHint: fmt.Sprintf("add a registration matching /%s/ to %s", pattern, registryDesc),
			})
		}
	}
	return vs
}

// valueWitness is one derived correspondence value with its source anchor.
type valueWitness struct {
	value string
	file  string
	line  int // 0 when path-derived
}

// deriveSide computes the ordered value set of one correspondence side.
// Files is a regex fully matching repo-relative paths; with Content set,
// values derive from content captures per matching file, merged over path
// captures.
func deriveSide(ctx *Context, id string, side *config.CaptureSide) []valueWitness {
	fileRe, err := regexp.Compile("^(?:" + side.Files + ")$")
	if err != nil {
		ctx.Warn("rule %s: invalid files regex: %v", id, err)
		return nil
	}
	var contentRe *regexp.Regexp
	if side.Content != "" {
		contentRe, err = regexp.Compile(side.Content)
		if err != nil {
			ctx.Warn("rule %s: invalid content regex: %v", id, err)
			return nil
		}
	}
	var out []valueWitness
	for _, f := range ctx.Tree.Files {
		match := fileRe.FindStringSubmatch(f.Path)
		if match == nil {
			continue
		}
		pathCaps := namedCaptures(fileRe, match)
		if contentRe == nil {
			out = append(out, valueWitness{value: expandValue(side.Value, pathCaps), file: f.Path})
			continue
		}
		content := ctx.Content(f)
		for _, idx := range contentRe.FindAllSubmatchIndex(content, -1) {
			caps := map[string]string{}
			for k, v := range pathCaps {
				caps[k] = v
			}
			for gi, name := range contentRe.SubexpNames() {
				if name == "" {
					continue
				}
				start, end := idx[2*gi], idx[2*gi+1]
				if start >= 0 {
					caps[name] = string(content[start:end])
				}
			}
			line := 1 + strings.Count(string(content[:idx[0]]), "\n")
			out = append(out, valueWitness{value: expandValue(side.Value, caps), file: f.Path, line: line})
		}
	}
	return out
}

// checkCorrespondence: the value set of Of must be subset-of (or equal-to)
// the value set of In.
func checkCorrespondence(ctx *Context, module, id string, sev report.Severity, rule config.ProvidesRule) []report.Violation {
	ofValues := deriveSide(ctx, id, rule.Of)
	inValues := deriveSide(ctx, id, rule.InSide)
	relation := rule.Relation
	if relation == "" {
		relation = "subset"
	}

	inSet := map[string]bool{}
	for _, w := range inValues {
		inSet[w.value] = true
	}
	ofSet := map[string]bool{}
	for _, w := range ofValues {
		ofSet[w.value] = true
	}

	var vs []report.Violation
	reported := map[string]bool{}
	for _, w := range ofValues {
		if inSet[w.value] || reported[w.value] {
			continue
		}
		reported[w.value] = true
		v := report.Violation{
			RuleID: id, Contract: report.ContractProvides,
			Blame: report.BlameProvider, Severity: sev,
			Path: w.file,
			Message: fmt.Sprintf("value %q (from %s) has no counterpart matching /%s/ (relation: %s)",
				w.value, w.file, rule.InSide.Files, relation),
			FixHint: fmt.Sprintf("create a counterpart matching /%s/ yielding %q, or remove %s",
				rule.InSide.Files, w.value, w.file),
		}
		if w.line > 0 {
			v.Line = report.IntPtr(w.line)
		}
		vs = append(vs, v)
	}
	if relation == "equal" {
		reportedIn := map[string]bool{}
		for _, w := range inValues {
			if ofSet[w.value] || reportedIn[w.value] {
				continue
			}
			reportedIn[w.value] = true
			v := report.Violation{
				RuleID: id, Contract: report.ContractProvides,
				Blame: report.BlameProvider, Severity: sev,
				Path: w.file,
				Message: fmt.Sprintf("value %q (from %s) has no counterpart matching /%s/ (relation: equal, reverse direction)",
					w.value, w.file, rule.Of.Files),
				FixHint: fmt.Sprintf("create a counterpart matching /%s/ yielding %q, or remove %s",
					rule.Of.Files, w.value, w.file),
			}
			if w.line > 0 {
				v.Line = report.IntPtr(w.line)
			}
			vs = append(vs, v)
		}
	}
	return vs
}
