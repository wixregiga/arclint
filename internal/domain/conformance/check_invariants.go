package conformance

import (
	"fmt"
	"path"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

const (
	declKindFunc      = "func"
	declKindMethod    = "method"
	declKindStruct    = "struct"
	declKindClass     = "class"
	declKindType      = "type"
	declKindInterface = "interface"
)

// evaluateInvariants judges one Module: every recorded domain contract
// in the four library shapes is visible in the Module's source as a
// named method (or constructor) called from its join points.
func evaluateInvariants(r rule.Rule, mem membership, obs Observations, knowledge vocab.UbiquitousLanguage) ([]Evaluation, error) {
	p, ok := r.Params().(rule.InvariantsParams)
	if !ok {
		return nil, fmt.Errorf("rule %s: invariants rule with %T params", r.ID(), r.Params())
	}
	var out []Evaluation
	for _, name := range sortedModules(r.Applicability().Modules()) {
		subject, err := rule.ModuleSubject(name)
		if err != nil {
			return nil, fmt.Errorf("invariants: %w", err)
		}
		if r.Applicability().ExcludedModule(name) {
			e, err := simpleEvaluation(r, subject, OutcomeNotApplicable)
			if err != nil {
				return nil, err
			}
			out = append(out, e)
			continue
		}
		idx := buildContractIndex(mem.moduleFiles[name], obs)
		vs, err := invariantsViolations(r, p, subject, idx, knowledge)
		if err != nil {
			return nil, err
		}
		e, err := completeEvaluation(r, subject, vs)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

type fileFacts struct {
	path  string
	lang  rule.Language
	pkg   string
	decls []Declaration
	calls []Call
}

type contractIndex struct {
	files []fileFacts
}

func buildContractIndex(paths []string, obs Observations) contractIndex {
	var files []fileFacts
	for _, p := range paths {
		facts, ok := obs.FactsFor(p)
		if !ok {
			continue
		}
		if facts.ParseFailure != "" {
			continue
		}
		if !facts.DeclarationsAvailable && !facts.CallsAvailable {
			continue
		}
		files = append(files, fileFacts{
			path:  p,
			lang:  facts.Language,
			pkg:   facts.Package,
			decls: facts.Declarations,
			calls: facts.Calls,
		})
	}
	return contractIndex{files: files}
}

func invariantsViolations(r rule.Rule, p rule.InvariantsParams, subject rule.Subject, idx contractIndex, knowledge vocab.UbiquitousLanguage) ([]Violation, error) {
	var vs []Violation
	for _, ctx := range knowledge.Contexts {
		for _, inv := range ctx.Invariants {
			more, err := checkInvariant(r, p, subject, idx, ctx, inv)
			if err != nil {
				return nil, err
			}
			vs = append(vs, more...)
		}
		for _, a := range ctx.Assertions {
			more, err := checkAssertion(r, subject, idx, a)
			if err != nil {
				return nil, err
			}
			vs = append(vs, more...)
		}
		for _, s := range ctx.Specifications {
			more, err := checkSpecification(r, subject, idx, s)
			if err != nil {
				return nil, err
			}
			vs = append(vs, more...)
		}
	}
	return vs, nil
}

func checkInvariant(r rule.Rule, p rule.InvariantsParams, subject rule.Subject, idx contractIndex, ctx vocab.BoundedContext, inv vocab.Invariant) ([]Violation, error) {
	_, vo := classifyRecordedOwner(ctx, inv.Owner)
	if vo && inv.ID == "" {
		return checkValueIntegrity(r, subject, idx, inv)
	}
	agg, _ := classifyRecordedOwner(ctx, inv.Owner)
	if agg && inv.ID != "" {
		return checkCluster(r, p, subject, idx, inv)
	}
	return nil, nil
}

func classifyRecordedOwner(ctx vocab.BoundedContext, owner string) (aggregate, valueObject bool) {
	for _, e := range ctx.Entities {
		if e.Name == owner {
			return e.Aggregate, false
		}
	}
	for _, v := range ctx.ValueObjects {
		if v.Name == owner {
			return false, true
		}
	}
	return false, false
}

func checkValueIntegrity(r rule.Rule, subject rule.Subject, idx contractIndex, inv vocab.Invariant) ([]Violation, error) {
	ctor, ok := findConstructor(idx, inv.Owner)
	if !ok {
		anchor, line := typeAnchor(idx, inv.Owner)
		v, err := newViolation(r, subject, anchor, line,
			fmt.Sprintf("value integrity %q: missing constructor for %s", inv.Statement, inv.Owner),
			fmt.Sprintf("add a constructor for %s (New/New%s, constructor, or __init__)", inv.Owner, inv.Owner))
		if err != nil {
			return nil, err
		}
		return []Violation{v}, nil
	}
	_ = ctor
	return nil, nil
}

func checkCluster(r rule.Rule, p rule.InvariantsParams, subject rule.Subject, idx contractIndex, inv vocab.Invariant) ([]Violation, error) {
	var vs []Violation
	methodByLang := map[rule.Language]string{}
	foundMethod := false
	var methodDecl locDecl
	for _, f := range idx.files {
		name, err := methodNameFor(inv.ID, f.lang)
		if err != nil {
			return nil, err
		}
		methodByLang[f.lang] = name
		if d, ok := findMethod(f, inv.Owner, name); ok {
			foundMethod = true
			methodDecl = locDecl{file: f, decl: d}
		}
	}
	if !foundMethod {
		anchor, line := typeAnchor(idx, inv.Owner)
		want := clusterMethodHint(inv.ID)
		v, err := newViolation(r, subject, anchor, line,
			fmt.Sprintf("cluster invariant %q: missing method %s on %s", inv.ID, want, inv.Owner),
			fmt.Sprintf("add method %s on %s", want, inv.Owner))
		if err != nil {
			return nil, err
		}
		return []Violation{v}, nil
	}
	_ = methodDecl

	ctors := findConstructors(idx, inv.Owner)
	if len(ctors) == 0 {
		anchor, line := typeAnchor(idx, inv.Owner)
		v, err := newViolation(r, subject, anchor, line,
			fmt.Sprintf("cluster invariant %q: missing constructor for %s", inv.ID, inv.Owner),
			fmt.Sprintf("add a constructor for %s that calls %s", inv.Owner, methodByLang[rule.LanguageGo]))
		if err != nil {
			return nil, err
		}
		vs = append(vs, v)
	}
	for _, ctor := range ctors {
		name := methodByLang[ctor.file.lang]
		if !calledFrom(ctor.file, ctor.decl.Name, name) {
			v, err := newViolation(r, subject, ctor.file.path, ctor.decl.StartLine,
				fmt.Sprintf("cluster invariant %q: constructor %s does not call %s", inv.ID, ctor.decl.Name, name),
				fmt.Sprintf("call %s from %s", name, ctor.decl.Name))
			if err != nil {
				return nil, err
			}
			vs = append(vs, v)
		}
	}

	commands := findCommands(idx, inv.Owner, methodByLang)
	for _, cmd := range commands {
		name := methodByLang[cmd.file.lang]
		if !calledFrom(cmd.file, cmd.decl.Name, name) {
			v, err := newViolation(r, subject, cmd.file.path, cmd.decl.StartLine,
				fmt.Sprintf("cluster invariant %q: command %s does not call %s", inv.ID, cmd.decl.Name, name),
				fmt.Sprintf("call %s from %s", name, cmd.decl.Name))
			if err != nil {
				return nil, err
			}
			vs = append(vs, v)
		}
	}

	if p.Closed {
		extras := findExtras(idx, inv.Owner, methodByLang)
		for _, extra := range extras {
			name := methodByLang[extra.file.lang]
			if calledFrom(extra.file, extra.decl.Name, name) {
				continue
			}
			v, err := newViolation(r, subject, extra.file.path, extra.decl.StartLine,
				fmt.Sprintf("cluster invariant %q: extra %s does not call %s", inv.ID, extra.decl.Name, name),
				fmt.Sprintf("call %s from %s, or drop closed", name, extra.decl.Name))
			if err != nil {
				return nil, err
			}
			vs = append(vs, v)
		}
	}
	return vs, nil
}

func checkAssertion(r rule.Rule, subject rule.Subject, idx contractIndex, a vocab.Assertion) ([]Violation, error) {
	var vs []Violation
	foundMethod := false
	foundOn := false
	var onDecl locDecl
	var methodName string
	for _, f := range idx.files {
		name, err := methodNameFor(a.ID, f.lang)
		if err != nil {
			return nil, err
		}
		if _, ok := findMethod(f, a.Owner, name); ok {
			foundMethod = true
			methodName = name
		}
		if d, ok := findMethod(f, a.Owner, a.On); ok {
			foundOn = true
			onDecl = locDecl{file: f, decl: d}
			methodName = name
		}
	}
	if !foundMethod {
		anchor, line := typeAnchor(idx, a.Owner)
		want := clusterMethodHint(a.ID)
		v, err := newViolation(r, subject, anchor, line,
			fmt.Sprintf("assertion %q: missing method %s on %s", a.ID, want, a.Owner),
			fmt.Sprintf("add method %s on %s", want, a.Owner))
		if err != nil {
			return nil, err
		}
		vs = append(vs, v)
	}
	if !foundOn {
		anchor, line := typeAnchor(idx, a.Owner)
		v, err := newViolation(r, subject, anchor, line,
			fmt.Sprintf("assertion %q: missing operation %s on %s", a.ID, a.On, a.Owner),
			fmt.Sprintf("add operation %s on %s that calls the assertion method", a.On, a.Owner))
		if err != nil {
			return nil, err
		}
		vs = append(vs, v)
		return vs, nil
	}
	if foundMethod && !calledFrom(onDecl.file, onDecl.decl.Name, methodName) {
		v, err := newViolation(r, subject, onDecl.file.path, onDecl.decl.StartLine,
			fmt.Sprintf("assertion %q: %s does not call %s", a.ID, a.On, methodName),
			fmt.Sprintf("call %s from %s", methodName, a.On))
		if err != nil {
			return nil, err
		}
		vs = append(vs, v)
	}
	return vs, nil
}

func checkSpecification(r rule.Rule, subject rule.Subject, idx contractIndex, s vocab.Specification) ([]Violation, error) {
	foundType := false
	foundSat := false
	var typePath string
	var typeLine int
	for _, f := range idx.files {
		if d, ok := findType(f, s.Name); ok {
			foundType = true
			typePath = f.path
			typeLine = d.StartLine
		}
		if hasSatisfaction(f, s.Name) {
			foundSat = true
		}
	}
	if !foundType {
		anchor := "."
		if len(idx.files) > 0 {
			anchor = path.Dir(idx.files[0].path)
		}
		v, err := newViolation(r, subject, anchor, 0,
			fmt.Sprintf("specification %q: missing type %s", s.Name, s.Name),
			fmt.Sprintf("add type %s with SatisfiedBy", s.Name))
		if err != nil {
			return nil, err
		}
		return []Violation{v}, nil
	}
	if !foundSat {
		v, err := newViolation(r, subject, typePath, typeLine,
			fmt.Sprintf("specification %q: missing satisfaction method on %s", s.Name, s.Name),
			fmt.Sprintf("add SatisfiedBy, satisfiedBy, or satisfied_by on %s", s.Name))
		if err != nil {
			return nil, err
		}
		return []Violation{v}, nil
	}
	return nil, nil
}

type locDecl struct {
	file fileFacts
	decl Declaration
}

func methodNameFor(id string, lang rule.Language) (string, error) {
	name, err := rule.CaseTerm(id, methodCase(lang))
	if err != nil {
		return "", fmt.Errorf("contract method name for %q: %w", id, err)
	}
	return name, nil
}

func methodCase(lang rule.Language) string {
	switch lang {
	case rule.LanguageGo:
		return "PascalCase"
	case rule.LanguageTypeScript:
		return "camelCase"
	case rule.LanguagePython:
		return "snake_case"
	default:
		return "PascalCase"
	}
}

func clusterMethodHint(id string) string {
	name, err := rule.CaseTerm(id, "PascalCase")
	if err != nil {
		return id
	}
	return name
}

func findType(f fileFacts, name string) (Declaration, bool) {
	for _, d := range f.decls {
		if d.Name != name {
			continue
		}
		switch d.Kind {
		case declKindStruct, declKindClass, declKindType, declKindInterface:
			return d, true
		}
	}
	return Declaration{}, false
}

func findMethod(f fileFacts, owner, name string) (Declaration, bool) {
	for _, d := range f.decls {
		if d.Kind == declKindMethod && d.Owner == owner && d.Name == name {
			return d, true
		}
	}
	return Declaration{}, false
}

func findConstructor(idx contractIndex, typeName string) (locDecl, bool) {
	ctors := findConstructors(idx, typeName)
	if len(ctors) == 0 {
		return locDecl{}, false
	}
	return ctors[0], true
}

func findConstructors(idx contractIndex, typeName string) []locDecl {
	var out []locDecl
	for _, f := range idx.files {
		for _, d := range f.decls {
			if isConstructor(d, typeName) {
				out = append(out, locDecl{file: f, decl: d})
			}
		}
	}
	return out
}

func isConstructor(d Declaration, typeName string) bool {
	if d.Kind == declKindMethod && d.Owner == typeName && (d.Name == "constructor" || d.Name == "__init__") {
		return true
	}
	if d.Kind != declKindFunc && d.Kind != declKindMethod {
		return false
	}
	if d.Name != "New" && d.Name != "create" && d.Name != "New"+typeName {
		return false
	}
	return resultsMention(d.Results, typeName)
}

func resultsMention(results []string, typeName string) bool {
	for _, r := range results {
		if typeMentions(r, typeName) {
			return true
		}
	}
	return false
}

func typeMentions(text, typeName string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == typeName || trimmed == "*"+typeName {
		return true
	}
	if strings.Contains(trimmed, typeName) {
		return true
	}
	return false
}

func findCommands(idx contractIndex, owner string, methods map[rule.Language]string) []locDecl {
	var out []locDecl
	for _, f := range idx.files {
		contract := methods[f.lang]
		for _, d := range f.decls {
			if d.Kind != declKindMethod || d.Owner != owner || !d.Exported {
				continue
			}
			if d.Name == contract || isConstructor(d, owner) {
				continue
			}
			if !hasErrorResult(d.Results, f.lang) {
				continue
			}
			out = append(out, locDecl{file: f, decl: d})
		}
	}
	return out
}

func findExtras(idx contractIndex, owner string, methods map[rule.Language]string) []locDecl {
	ownerFiles := map[string]bool{}
	for _, f := range idx.files {
		if _, ok := findType(f, owner); ok {
			ownerFiles[f.path] = true
			if f.pkg != "" {
				for _, other := range idx.files {
					if other.pkg == f.pkg {
						ownerFiles[other.path] = true
					}
				}
			}
		}
	}
	var out []locDecl
	for _, f := range idx.files {
		if !ownerFiles[f.path] {
			continue
		}
		contract := methods[f.lang]
		for _, d := range f.decls {
			if d.Kind != declKindFunc && d.Kind != declKindMethod {
				continue
			}
			if !d.Exported {
				continue
			}
			if !hasErrorResult(d.Results, f.lang) {
				continue
			}
			if d.Kind == declKindMethod && d.Owner == owner {
				continue
			}
			if isConstructor(d, owner) {
				continue
			}
			if d.Name == contract {
				continue
			}
			out = append(out, locDecl{file: f, decl: d})
		}
	}
	return out
}

func hasErrorResult(results []string, lang rule.Language) bool {
	for _, r := range results {
		switch lang {
		case rule.LanguageGo:
			if r == "error" {
				return true
			}
		case rule.LanguageTypeScript:
			if strings.Contains(r, "Error") {
				return true
			}
		case rule.LanguagePython:
			if strings.Contains(r, "Exception") || strings.Contains(r, "Error") {
				return true
			}
		}
	}
	return false
}

func calledFrom(f fileFacts, enclosing, callee string) bool {
	for _, c := range f.calls {
		if c.Enclosing == enclosing && c.Callee == callee {
			return true
		}
	}
	return false
}

func hasSatisfaction(f fileFacts, typeName string) bool {
	for _, d := range f.decls {
		if d.Kind != declKindMethod || d.Owner != typeName {
			continue
		}
		switch d.Name {
		case "SatisfiedBy", "satisfiedBy", "satisfied_by":
			return true
		}
	}
	return false
}

func typeAnchor(idx contractIndex, typeName string) (string, int) {
	for _, f := range idx.files {
		if d, ok := findType(f, typeName); ok {
			return f.path, d.StartLine
		}
	}
	if len(idx.files) > 0 {
		return path.Dir(idx.files[0].path), 0
	}
	return ".", 0
}
