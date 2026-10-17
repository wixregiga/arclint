package conformance

import (
	"fmt"
	"sort"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/rule"
)

// ServiceImplementation is one implementation of a domain service
// declared as an interface or an abstract class, with what its
// operation must do to reach the guarantee's check in the model and
// how the check judges it.
type ServiceImplementation struct {
	// Type is the implementing type.
	Type string
	// Operation is the operation as the implementation declares it.
	Operation string
	// Path and Line locate the operation's declaration, or the type's
	// when the operation is not located.
	Path string
	Line int
	// Evidence is how the language established the implementation.
	Evidence ImplementationEvidence
	// Must says what the operation does to reach the check: call the
	// context's function, return the value object, or either.
	Must string
	// Outcome is conforms, violates, or undetermined.
	Outcome Outcome
	// Reason says why the outcome is undetermined, or what the finding
	// is.
	Reason string
}

// validate enforces that a judged implementation names its type and
// its place, and states why it does not conform.
func (s ServiceImplementation) validate() error {
	if strings.TrimSpace(s.Type) == "" || strings.TrimSpace(s.Operation) == "" || strings.TrimSpace(s.Path) == "" || s.Line <= 0 {
		return fmt.Errorf("implementation: type, operation, path, and positive line are required")
	}
	if !s.Evidence.Valid() {
		return fmt.Errorf("implementation %s: evidence %q invalid", s.Type, s.Evidence)
	}
	switch s.Outcome {
	case OutcomeConforms:
	case OutcomeViolates, OutcomeUndetermined:
		if strings.TrimSpace(s.Reason) == "" {
			return fmt.Errorf("implementation %s: %s without a reason", s.Type, s.Outcome)
		}
	default:
		return fmt.Errorf("implementation %s: outcome %q is not conforms, violates, or undetermined", s.Type, s.Outcome)
	}
	return nil
}

// Implementations judges every implementation of the domain service
// the owner names against the check of the assertion under the key, as
// assertion/checked-by-its-operation judges it; nil when the owner is
// not a located service declared as an interface or an abstract class,
// or the model holds no check of the guarantee.
func (c Carriers) Implementations(ctx, owner, key string) ([]ServiceImplementation, error) {
	cc, ok := c.contexts[ctx]
	if !ok {
		return nil, nil
	}
	as, aggregate, recorded := cc.recordedAssertion(owner, key)
	loc := cc.terms[owner]
	if !recorded || aggregate || !loc.located() || bodiless(loc.decl().decl) == "" {
		return nil, nil
	}
	service := loc.decl()
	idx := cc.unitIdx(service)
	op, hasOp, err := idx.methodNamed(service.decl.Name, as.On)
	if err != nil || !hasOp {
		return nil, err
	}
	ops, err := idx.methodsNamed(service.decl.Name, as.On)
	if err != nil {
		return nil, err
	}
	check, err := cc.modelCheck(service, ops, key)
	if err != nil || !check.found {
		return nil, err
	}
	judged, _ := c.code.implementationsOf(service, op, cc.reachOf(service, ops, key))
	out := make([]ServiceImplementation, 0, len(judged))
	for _, j := range judged {
		out = append(out, j.ServiceImplementation)
	}
	return out, nil
}

// modelReach is what an implementation's operation may do to reach the
// check of a declared service's guarantee in the model: call the
// function of the context spelled assert followed by the key, or return
// the value object whose invariant of the same key checks it.
type modelReach struct {
	fn, value       locDecl
	hasFn, hasValue bool
}

// reachOf finds the checks of a guarantee in the model that an
// implementation may reach; ops are the service's declarations of the
// operation.
func (cc contextCode) reachOf(service locDecl, ops []locDecl, key string) modelReach {
	var reach modelReach
	reach.fn, reach.hasFn = cc.contextFunction(service, assertKey(key))
	if v := cc.valueCheck(ops, key); v.found {
		reach.value, reach.hasValue = v.at, true
	}
	return reach
}

// must spells what an implementation's operation does to reach the
// check.
func (r modelReach) must() string {
	switch {
	case r.hasFn && r.hasValue:
		return fmt.Sprintf("call %s or return %s", r.fn.decl.Name, r.value.decl.Name)
	case r.hasFn:
		return "call " + r.fn.decl.Name
	default:
		return "return " + r.value.decl.Name
	}
}

// judgedImplementation is one implementation as judged, with the
// declaration a finding anchors at.
type judgedImplementation struct {
	ServiceImplementation
	at locDecl
}

// implementationsOf judges every implementation of a declared domain
// service the implementations fact shows, outside test files, where a
// test double stands in for one. The second result says where more may
// exist that the facts cannot show: a file of the service's language
// that does not parse or supplies no implementations, or a type whose
// abstractions resolution cannot follow.
func (code domainCode) implementationsOf(service, op locDecl, reach modelReach) ([]judgedImplementation, []string) {
	var out []judgedImplementation
	var open []string
	lang := service.file.lang
	for _, g := range code.all.sourceGaps() {
		if rule.LanguageOf(g.path) == lang {
			open = append(open, fmt.Sprintf("%s: %s, so an implementation of %s there is not seen", g.path, g.reason, service.decl.Name))
		}
	}
	for _, f := range code.all.files {
		if f.lang != lang || isTestFile(f.path) {
			continue
		}
		if !f.implementations {
			open = append(open, fmt.Sprintf("%s: its implementations are not available, so an implementation of %s there is not seen", f.path, service.decl.Name))
			continue
		}
		unknown := map[string][]string{}
		for _, d := range f.decls {
			if d.Abstract {
				// An abstract class has no instance of its own; the classes
				// deriving it are judged.
				continue
			}
			for _, impl := range d.Implements {
				switch {
				case service.declaredBy(impl.Abstraction):
					out = append(out, code.judgeImplementation(locDecl{file: f, decl: d}, impl, service, op, reach))
				case impl.Abstraction.Resolution != ResolutionUndetermined:
				case impl.Abstraction.Name == service.decl.Name:
					open = append(open, fmt.Sprintf("%s:%d: %s names %s, which resolution cannot follow: %s", f.path, d.StartLine, d.Name, impl.Abstraction.Name, impl.Reason))
				case impl.Abstraction.Name == "":
					unknown[impl.Reason] = append(unknown[impl.Reason], d.Name)
				}
			}
		}
		reasons := make([]string, 0, len(unknown))
		for reason := range unknown {
			reasons = append(reasons, reason)
		}
		sort.Strings(reasons)
		for _, reason := range reasons {
			open = append(open, fmt.Sprintf("%s: what %s implement is undetermined: %s", f.path, strings.Join(unknown[reason], ", "), reason))
		}
	}
	return out, open
}

// judgeImplementation judges one implementation: its operation, where
// the type's own lookup reaches it, makes a resolved call to the
// context's function, or every declaration of it returns the value
// object. A match of member names alone, or a registration, cannot
// prove a violation, so either leaves the implementation undetermined
// when it does not reach the check.
func (code domainCode) judgeImplementation(typ locDecl, impl Implementation, service, op locDecl, reach modelReach) judgedImplementation {
	j := judgedImplementation{at: typ, ServiceImplementation: ServiceImplementation{
		Type: typ.decl.Name, Operation: op.decl.Name, Path: typ.file.path, Line: typ.decl.StartLine,
		Evidence: impl.Evidence, Must: reach.must(),
	}}
	undetermined := func(reason string) judgedImplementation {
		j.Outcome, j.Reason = OutcomeUndetermined, reason
		return j
	}
	var reached Reference
	for _, ref := range impl.Operations {
		if ref.Name == op.decl.Name {
			reached = ref
		}
	}
	switch reached.Resolution {
	case "":
		return undetermined(fmt.Sprintf("the facts name no operation %s of %s", op.decl.Name, typ.decl.Name))
	case ResolutionUndetermined:
		return undetermined(fmt.Sprintf("%s of %s cannot be followed: %s", op.decl.Name, typ.decl.Name, reached.Reason))
	case ResolutionExternal, ResolutionLocal:
		return undetermined(fmt.Sprintf("%s of %s is declared outside the repository", op.decl.Name, typ.decl.Name))
	case ResolutionDeclared:
	}
	f, decls, holder := code.all.operation(reached)
	if len(decls) == 0 {
		return undetermined(fmt.Sprintf("%s of %s is declared in %s, whose declarations are not supplied", op.decl.Name, typ.decl.Name, reached.File))
	}
	j.Operation, j.Path, j.Line, j.at = decls[0].decl.Name, f.path, decls[0].decl.StartLine, decls[0]
	if holder.Kind == declKindInterface {
		return undetermined(fmt.Sprintf("%s of %s is promoted from the interface %s it embeds, and the implementation that value holds is not declared", op.decl.Name, typ.decl.Name, holder.Name))
	}
	reaches, open := reach.reachedBy(f, decls, reached)
	switch {
	case reaches:
		j.Outcome = OutcomeConforms
		return j
	case impl.Evidence != ImplementationExact:
		return undetermined(fmt.Sprintf("%s does not %s, but it implements %s only as far as this shows: %s", qualified(reached), reach.must(), service.decl.Name, impl.Reason))
	case open != "":
		return undetermined(open)
	}
	j.Outcome = OutcomeViolates
	j.Reason = fmt.Sprintf("implementation %s of declared service %s: %s does not %s", typ.decl.Name, service.decl.Name, qualified(reached), reach.must())
	return j
}

// reachedBy reports whether an operation reaches the check: a call
// from its body that resolution reached at the context's function, or
// a result every declaration of it names as the value object. The
// second result says why it may reach the check unseen: a call or a
// result resolution could not follow.
func (r modelReach) reachedBy(f fileFacts, decls []locDecl, op Reference) (bool, string) {
	open := ""
	note := func(reason string) {
		if open == "" {
			open = reason
		}
	}
	if r.hasValue {
		returns := true
		for _, d := range decls {
			value, unresolved, found := returnedValue(d, []locDecl{r.value})
			switch {
			case unresolved != "":
				note(unresolved)
				returns = false
			case !found || !sameDecl(value, r.value):
				returns = false
			}
		}
		if returns {
			return true, ""
		}
	}
	if !r.hasFn {
		return false, open
	}
	if !f.resolvedCalls {
		return false, fmt.Sprintf("%s: the calls of %s: the language supplied no resolved calls for the file", f.path, qualified(op))
	}
	for _, c := range f.calls {
		if c.Enclosing != op.Name || c.EnclosingOwner != op.Owner {
			continue
		}
		t := c.Target
		if t.Resolution == ResolutionDeclared && t.Owner == "" && t.File == r.fn.file.path && t.Name == r.fn.decl.Name {
			return true, ""
		}
		if t.Resolution == ResolutionUndetermined {
			note(unresolvedAt(f.path, c.Line, fmt.Sprintf("call %s in %s", c.Callee, qualified(op)), t))
		}
	}
	return false, open
}

// operation finds the declarations of the method a reference reaches,
// overloads included, with the file holding them and the declaration
// of the type that declares them.
func (idx contractIndex) operation(ref Reference) (fileFacts, []locDecl, Declaration) {
	for _, f := range idx.files {
		if f.path != ref.File {
			continue
		}
		var decls []locDecl
		var holder Declaration
		for _, d := range f.decls {
			switch {
			case d.Kind == declKindMethod && d.Owner == ref.Owner && d.Name == ref.Name:
				decls = append(decls, locDecl{file: f, decl: d})
			case d.Owner == "" && d.Name == ref.Owner && isTypeKind(d.Kind):
				holder = d
			}
		}
		return f, decls, holder
	}
	return fileFacts{}, nil, Declaration{}
}

// qualified spells a method as Owner.Name.
func qualified(ref Reference) string {
	if ref.Owner == "" {
		return ref.Name
	}
	return ref.Owner + "." + ref.Name
}
