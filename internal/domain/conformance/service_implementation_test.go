package conformance_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

const (
	implModelFile   = "user/password.go"
	implAdapterFile = "adapters/encrypter.go"
)

// implementationKnowledge records the Encrypter service with its
// guarantee on Encrypt, and the value object whose invariant has the
// same key.
func implementationKnowledge(t *testing.T, verifiedBy ...string) vocab.UbiquitousLanguage {
	t.Helper()
	return oneContext(t, vocab.BoundedContext{
		Name: "identity", Definition: "d", Line: 1,
		ValueObjects: []vocab.ValueObject{
			{Name: "EncryptedPassword", Definition: "d", Line: 2, Invariants: []vocab.Invariant{{Key: "not-empty", Statement: "s", Line: 3}}},
		},
		Services: []vocab.DomainService{{
			Name: "Encrypter", Definition: "d", Line: 4,
			Assertions: []vocab.Assertion{{Key: "not-empty", On: "Encrypt", Statement: "s", Line: 5, VerifiedBy: verifiedBy}},
		}},
	})
}

// implementationModel is the model's file: the value object, the
// Encrypter interface whose Encrypt returns result, and, when fn is
// set, the function AssertNotEmpty.
func implementationModel(result conformance.Reference, fn bool) conformance.LanguageFacts {
	decls := []conformance.Declaration{
		{Kind: "struct", Name: "EncryptedPassword", Exported: true, StartLine: 3},
		{Kind: "func", Name: "NewEncryptedPassword", Exported: true, StartLine: 6, Results: []string{"EncryptedPassword", "error"},
			Type: declared(implModelFile, "", "EncryptedPassword")},
		{Kind: "interface", Name: "Encrypter", Exported: true, StartLine: 12},
		{Kind: "method", Name: "Encrypt", Owner: "Encrypter", Exported: true, StartLine: 13, Type: result},
	}
	if fn {
		decls = append(decls, conformance.Declaration{Kind: "func", Name: "AssertNotEmpty", Exported: true, StartLine: 20, Type: external("", "", "error")})
	}
	f := goDecls("user", decls)
	f.ImplementationsAvailable, f.ResolvedCallsAvailable = true, true
	return f
}

// implementationAdapter is the adapter's file: the type Plain whose
// Encrypt, at line 5, makes the calls given and returns result, and
// implements Encrypter with the evidence and reason given.
func implementationAdapter(result conformance.Reference, evidence conformance.ImplementationEvidence, reason string, calls ...conformance.Call) conformance.LanguageFacts {
	impl := conformance.Implementation{
		Abstraction: declared(implModelFile, "", "Encrypter"), Evidence: evidence, Reason: reason,
		Operations: []conformance.Reference{declared(implAdapterFile, "Plain", "Encrypt")},
	}
	f := goDecls("adapters", []conformance.Declaration{
		{Kind: "struct", Name: "Plain", Exported: true, StartLine: 3, Implements: []conformance.Implementation{impl}},
		{Kind: "method", Name: "Encrypt", Owner: "Plain", Exported: true, StartLine: 5, EndLine: 7, Type: result},
	}, calls...)
	f.ImplementationsAvailable, f.ResolvedCallsAvailable = true, true
	return f
}

// withStaticTwin adds to the adapter a static Encrypt, at lines 8 to 10,
// whose body calls the check: another operation than the instance one.
func withStaticTwin(f conformance.LanguageFacts, check conformance.Reference) conformance.LanguageFacts {
	f.Declarations = append(f.Declarations, conformance.Declaration{Kind: "method", Name: "Encrypt", Owner: "Plain", Exported: true, Static: true, StartLine: 8, EndLine: 10, Type: f.Declarations[1].Type})
	f.Calls = append(f.Calls, conformance.Call{Callee: check.Name, Line: 9, Enclosing: "Encrypt", EnclosingOwner: "Plain", Target: check})
	return f
}

func callTo(target conformance.Reference) conformance.Call {
	return conformance.Call{Callee: target.Name, Line: 6, Enclosing: "Encrypt", EnclosingOwner: "Plain", Target: target}
}

// judgedImplementations runs the rule and returns its outcome, its
// findings as path:line message, its undetermined reasons, and the
// implementations the coverage of the guarantee lists.
func judgedImplementations(t *testing.T, facts map[string]conformance.LanguageFacts, knowledge vocab.UbiquitousLanguage) (conformance.Outcome, []string, []string, []conformance.ServiceImplementation) {
	t.Helper()
	paths := make([]string, 0, len(facts))
	for p := range facts {
		paths = append(paths, p)
	}
	id := "assertion/checked-by-its-operation"
	a := runOne(t, builtInRule(t, id), nil, observed(paths...), facts, knowledge)
	var findings, reasons []string
	for _, v := range a.Violations() {
		findings = append(findings, v.Path()+":"+strconv.Itoa(v.Line())+" "+v.Message())
	}
	for _, e := range a.Evaluations() {
		reasons = append(reasons, e.Unresolved()...)
	}
	var impls []conformance.ServiceImplementation
	for _, c := range a.Contracts() {
		if c.Owner() == "Encrypter" {
			impls = c.Implementations()
		}
	}
	return outcomesOf(a)[id+"|context:identity"], findings, reasons, impls
}

// Every implementation of a declared service must reach the check in
// the model: its operation calls the function of the context, as
// resolution reached it, or returns the value object. One that does
// neither is the finding, at its operation; one whose evidence or
// calls cannot prove it does neither is undetermined, with the reason;
// with none observed the guarantee is undetermined unless verified_by
// names its tests.
func TestEveryImplementationReachesTheCheck(t *testing.T) {
	text := external("", "", "string")
	value := declared(implModelFile, "", "EncryptedPassword")
	check := declared(implModelFile, "", "AssertNotEmpty")
	exact := conformance.ImplementationExact
	for _, tc := range []struct {
		name     string
		facts    map[string]conformance.LanguageFacts
		verified []string
		outcome  conformance.Outcome
		finding  string
		reason   string
		listed   conformance.Outcome
	}{
		{name: "the operation calls the function", outcome: conformance.OutcomeConforms, listed: conformance.OutcomeConforms,
			facts: map[string]conformance.LanguageFacts{implModelFile: implementationModel(text, true), implAdapterFile: implementationAdapter(text, exact, "", callTo(check))}},
		{name: "the operation skips the function", outcome: conformance.OutcomeViolates, listed: conformance.OutcomeViolates,
			finding: implAdapterFile + ":5 implementation Plain of declared service Encrypter: Plain.Encrypt does not call AssertNotEmpty",
			facts:   map[string]conformance.LanguageFacts{implModelFile: implementationModel(text, true), implAdapterFile: implementationAdapter(text, exact, "", callTo(external("strings", "", "ToUpper")))}},
		{name: "a call resolution cannot follow", outcome: conformance.OutcomeUndetermined, listed: conformance.OutcomeUndetermined,
			reason: "call hidden in Plain.Encrypt: hidden is a function value",
			facts: map[string]conformance.LanguageFacts{implModelFile: implementationModel(text, true), implAdapterFile: implementationAdapter(text, exact, "",
				conformance.Call{Callee: "hidden", Line: 6, Enclosing: "Encrypt", EnclosingOwner: "Plain", Target: undetermined("hidden is a function value")})}},
		{name: "the operation returns the value", outcome: conformance.OutcomeConforms, listed: conformance.OutcomeConforms,
			facts: map[string]conformance.LanguageFacts{implModelFile: implementationModel(value, false), implAdapterFile: implementationAdapter(value, exact, "")}},
		{name: "the operation returns something else", outcome: conformance.OutcomeViolates, listed: conformance.OutcomeViolates,
			finding: implAdapterFile + ":5 implementation Plain of declared service Encrypter: Plain.Encrypt does not return EncryptedPassword",
			facts:   map[string]conformance.LanguageFacts{implModelFile: implementationModel(value, false), implAdapterFile: implementationAdapter(text, exact, "")}},
		{name: "a match of member names alone", outcome: conformance.OutcomeUndetermined, listed: conformance.OutcomeUndetermined,
			reason: "implements Encrypter only as far as this shows: member names alone",
			facts:  map[string]conformance.LanguageFacts{implModelFile: implementationModel(text, true), implAdapterFile: implementationAdapter(text, conformance.ImplementationHeuristic, "member names alone")}},
		{name: "a match of member names alone that calls the function", outcome: conformance.OutcomeUndetermined, listed: conformance.OutcomeConforms,
			reason: "no implementation of it is established",
			facts:  map[string]conformance.LanguageFacts{implModelFile: implementationModel(text, true), implAdapterFile: implementationAdapter(text, conformance.ImplementationHeuristic, "member names alone", callTo(check))}},
		{name: "a static method of the same name calls the function", outcome: conformance.OutcomeViolates, listed: conformance.OutcomeViolates,
			finding: implAdapterFile + ":5 implementation Plain of declared service Encrypter: Plain.Encrypt does not call AssertNotEmpty",
			facts:   map[string]conformance.LanguageFacts{implModelFile: implementationModel(text, true), implAdapterFile: withStaticTwin(implementationAdapter(text, exact, ""), check)}},
		{name: "a registration", outcome: conformance.OutcomeUndetermined, listed: conformance.OutcomeUndetermined,
			reason: "registered through ABCMeta.register",
			facts:  map[string]conformance.LanguageFacts{implModelFile: implementationModel(text, true), implAdapterFile: implementationAdapter(text, conformance.ImplementationUndetermined, "registered through ABCMeta.register")}},
		{name: "no implementation observed", outcome: conformance.OutcomeUndetermined,
			reason: "declared service Encrypter: no implementation of it is established, so whether one reaches the check of not-empty is undetermined",
			facts:  map[string]conformance.LanguageFacts{implModelFile: implementationModel(text, true)}},
		{name: "no implementation observed, verified by a test", outcome: conformance.OutcomeConforms, verified: []string{"user/password_test.go:TestNotEmpty"},
			facts: map[string]conformance.LanguageFacts{implModelFile: implementationModel(text, true)}},
		{name: "only a test double implements it", outcome: conformance.OutcomeUndetermined,
			reason: "no implementation of it is established",
			facts:  map[string]conformance.LanguageFacts{implModelFile: implementationModel(text, true), "adapters/encrypter_test.go": implementationAdapter(text, exact, "")}},
		{name: "a file whose implementations are not available", outcome: conformance.OutcomeUndetermined, listed: conformance.OutcomeConforms,
			reason: "other/other.go: its implementations are not available, so an implementation of Encrypter there is not seen",
			facts: map[string]conformance.LanguageFacts{implModelFile: implementationModel(text, true), implAdapterFile: implementationAdapter(text, exact, "", callTo(check)),
				"other/other.go": goDecls("other", []conformance.Declaration{{Kind: "struct", Name: "Other", StartLine: 1}})}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if p, ok := tc.facts["adapters/encrypter_test.go"]; ok {
				// The double's facts name the test file as the one holding
				// its operation.
				p.Declarations[0].Implements[0].Operations = []conformance.Reference{declared("adapters/encrypter_test.go", "Plain", "Encrypt")}
				tc.facts["adapters/encrypter_test.go"] = p
			}
			outcome, findings, reasons, impls := judgedImplementations(t, tc.facts, implementationKnowledge(t, tc.verified...))
			if outcome != tc.outcome {
				t.Fatalf("outcome %s, want %s\nfindings %q\nreasons %q", outcome, tc.outcome, findings, reasons)
			}
			if tc.finding != "" && (len(findings) != 1 || findings[0] != tc.finding) {
				t.Errorf("findings %q, want %q", findings, tc.finding)
			}
			if tc.finding == "" && len(findings) != 0 {
				t.Errorf("unexpected findings %q", findings)
			}
			if tc.reason != "" && !strings.Contains(strings.Join(reasons, "\n"), tc.reason) {
				t.Errorf("reasons %q lack %q", reasons, tc.reason)
			}
			if tc.listed == "" {
				if len(impls) != 0 {
					t.Errorf("coverage lists implementations %+v", impls)
				}
				return
			}
			if len(impls) != 1 || impls[0].Type != "Plain" || impls[0].Outcome != tc.listed || impls[0].Path != implAdapterFile || impls[0].Line != 5 {
				t.Errorf("coverage lists %+v, want Plain %s at %s:5", impls, tc.listed, implAdapterFile)
			}
		})
	}
}

// An implementation whose operation is promoted from an interface it
// embeds holds some other value's implementation, which no declaration
// names, so whether it reaches the check is undetermined.
func TestPromotedFromAnEmbeddedInterfaceIsUndetermined(t *testing.T) {
	adapter := implementationAdapter(external("", "", "string"), conformance.ImplementationExact, "")
	adapter.Declarations[0].Implements[0].Operations = []conformance.Reference{declared(implModelFile, "Encrypter", "Encrypt")}
	facts := map[string]conformance.LanguageFacts{implModelFile: implementationModel(external("", "", "string"), true), implAdapterFile: adapter}
	outcome, findings, reasons, _ := judgedImplementations(t, facts, implementationKnowledge(t))
	if outcome != conformance.OutcomeUndetermined || len(findings) != 0 || !strings.Contains(strings.Join(reasons, "\n"), "promoted from the interface Encrypter it embeds") {
		t.Errorf("outcome %s findings %q reasons %q", outcome, findings, reasons)
	}
}

// The implementations fact states its evidence: exact without a reason,
// heuristic and undetermined with one, and only on a type.
func TestImplementationsStateTheirEvidence(t *testing.T) {
	good := implementationAdapter(external("", "", "string"), conformance.ImplementationExact, "")
	for _, tc := range []struct {
		name string
		edit func(*conformance.LanguageFacts)
	}{
		{"exact with a reason", func(f *conformance.LanguageFacts) { f.Declarations[0].Implements[0].Reason = "names match" }},
		{"heuristic without a reason", func(f *conformance.LanguageFacts) {
			f.Declarations[0].Implements[0].Evidence = conformance.ImplementationHeuristic
		}},
		{"exact for an unresolved abstraction", func(f *conformance.LanguageFacts) {
			f.Declarations[0].Implements[0].Abstraction = undetermined("no such type")
		}},
		{"an unknown evidence", func(f *conformance.LanguageFacts) { f.Declarations[0].Implements[0].Evidence = "likely" }},
		{"on a method", func(f *conformance.LanguageFacts) {
			f.Declarations[1].Implements = f.Declarations[0].Implements
		}},
		{"not supplied", func(f *conformance.LanguageFacts) { f.ImplementationsAvailable = false }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := good
			f.Declarations = append([]conformance.Declaration(nil), good.Declarations...)
			f.Declarations[0].Implements = append([]conformance.Implementation(nil), good.Declarations[0].Implements...)
			tc.edit(&f)
			files := observed(implModelFile, implAdapterFile)
			if _, err := conformance.NewObservations(files, map[string]conformance.LanguageFacts{implModelFile: implementationModel(external("", "", "string"), true), implAdapterFile: f}); err == nil {
				t.Error("accepted")
			}
		})
	}
	if _, err := conformance.NewObservations(observed(implModelFile, implAdapterFile), map[string]conformance.LanguageFacts{implModelFile: implementationModel(external("", "", "string"), true), implAdapterFile: good}); err != nil {
		t.Errorf("rejected a valid implementation: %v", err)
	}
	if !good.Supports(rule.FactImplementations) {
		t.Error("supplied implementations are not supported")
	}
}

// Every function of the context spelled from the key checks the
// guarantee: two services of one context, each with the check in its
// own package, are each reached by an implementation calling its own.
func TestAnyCheckOfTheContextIsReached(t *testing.T) {
	knowledge := oneContext(t, vocab.BoundedContext{
		Name: "identity", Definition: "d", Line: 1,
		Services: []vocab.DomainService{
			{Name: "Encrypter", Definition: "d", Line: 2, Assertions: []vocab.Assertion{{Key: "not-empty", On: "Encrypt", Statement: "s", Line: 3}}},
			{Name: "TokenIssuer", Definition: "d", Line: 4, Assertions: []vocab.Assertion{{Key: "not-empty", On: "Issue", Statement: "s", Line: 5}}},
		},
	})
	text := external("", "", "string")
	tokenFile := "token/token.go"
	token := goDecls("token", []conformance.Declaration{
		{Kind: "interface", Name: "TokenIssuer", Exported: true, StartLine: 3},
		{Kind: "method", Name: "Issue", Owner: "TokenIssuer", Exported: true, StartLine: 4, Type: text},
		{Kind: "func", Name: "AssertNotEmpty", Exported: true, StartLine: 8, Type: external("", "", "error")},
	})
	token.ImplementationsAvailable, token.ResolvedCallsAvailable = true, true
	issuerFile := "adapters/issuer.go"
	issuer := goDecls("adapters", []conformance.Declaration{
		{Kind: "struct", Name: "Issuer", Exported: true, StartLine: 3, Implements: []conformance.Implementation{{
			Abstraction: declared(tokenFile, "", "TokenIssuer"), Evidence: conformance.ImplementationExact,
			Operations: []conformance.Reference{declared(issuerFile, "Issuer", "Issue")},
		}}},
		{Kind: "method", Name: "Issue", Owner: "Issuer", Exported: true, StartLine: 5, EndLine: 7, Type: text},
	}, conformance.Call{Callee: "AssertNotEmpty", Line: 6, Enclosing: "Issue", EnclosingOwner: "Issuer", Target: declared(tokenFile, "", "AssertNotEmpty")})
	issuer.ImplementationsAvailable, issuer.ResolvedCallsAvailable = true, true
	facts := map[string]conformance.LanguageFacts{
		implModelFile:   implementationModel(text, true),
		implAdapterFile: implementationAdapter(text, conformance.ImplementationExact, "", callTo(declared(implModelFile, "", "AssertNotEmpty"))),
		tokenFile:       token,
		issuerFile:      issuer,
	}
	outcome, findings, reasons, _ := judgedImplementations(t, facts, knowledge)
	if outcome != conformance.OutcomeConforms || len(findings) != 0 {
		t.Errorf("outcome %s findings %q reasons %q", outcome, findings, reasons)
	}
}

// A static method of the operation's name written on the operation's
// own line leaves a call there unattributable, so a call to the check
// on that line proves nothing.
func TestACallSharingALineWithAStaticTwinIsUndetermined(t *testing.T) {
	text := external("", "", "string")
	check := declared(implModelFile, "", "AssertNotEmpty")
	adapter := implementationAdapter(text, conformance.ImplementationExact, "",
		conformance.Call{Callee: "AssertNotEmpty", Line: 5, Enclosing: "Encrypt", EnclosingOwner: "Plain", Target: check})
	adapter.Declarations[1].EndLine = 5
	adapter.Declarations = append(adapter.Declarations, conformance.Declaration{Kind: "method", Name: "Encrypt", Owner: "Plain", Exported: true, Static: true, StartLine: 5, EndLine: 5, Type: text})
	facts := map[string]conformance.LanguageFacts{implModelFile: implementationModel(text, true), implAdapterFile: adapter}
	outcome, findings, reasons, _ := judgedImplementations(t, facts, implementationKnowledge(t))
	if outcome != conformance.OutcomeUndetermined || len(findings) != 0 || !strings.Contains(strings.Join(reasons, "\n"), "shares the line with a static method") {
		t.Errorf("outcome %s findings %q reasons %q", outcome, findings, reasons)
	}
}
