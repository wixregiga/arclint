package rule

import (
	"fmt"
	"strings"
)

// Language is a source language ArcLint recognizes and can analyze.
// Recognition does not imply coverage by every Rule.
type Language string

// The recognized source languages.
const (
	LanguageGo         Language = "go"
	LanguageTypeScript Language = "typescript"
	LanguagePython     Language = "python"
)

// Languages returns the supported set in stable order.
func Languages() []Language {
	return []Language{LanguageGo, LanguageTypeScript, LanguagePython}
}

// Valid reports whether the value is a supported language.
func (l Language) Valid() bool {
	for _, known := range Languages() {
		if l == known {
			return true
		}
	}
	return false
}

// LanguageOf maps a repo-relative file path to the Programming Language
// whose facts describe it, or "" for files no language owns. Go owns
// .go, TypeScript owns .ts and .tsx, Python owns .py.
func LanguageOf(path string) Language {
	switch {
	case strings.HasSuffix(path, ".go"):
		return LanguageGo
	case strings.HasSuffix(path, ".ts"), strings.HasSuffix(path, ".tsx"):
		return LanguageTypeScript
	case strings.HasSuffix(path, ".py"):
		return LanguagePython
	default:
		return ""
	}
}

// Fact names one normalized fact class Enforcement may require.
type Fact string

const (
	// FactFileTree is the walked repository file list.
	FactFileTree Fact = "file_tree"
	// FactImports is the classified per-file import view.
	FactImports Fact = "imports"
	// FactDeclarations is the normalized per-file declaration view.
	FactDeclarations Fact = "declarations"
	// FactCalls is the normalized per-file call view: callee
	// identifier, line, and enclosing func, parser-exact.
	FactCalls Fact = "calls"
)

// Assurance is the strength of conclusion justified by Enforcement
// evidence. It is independent from Rule Severity; unsupported is an
// Evaluation Outcome, never an Assurance value.
type Assurance string

const (
	// AssuranceExact fully decides the Claim within a documented
	// analysis limit.
	AssuranceExact Assurance = "exact"
	// AssurancePartial means reported Violations are trustworthy but
	// some may be unobservable.
	AssurancePartial Assurance = "partial"
	// AssuranceHeuristic may produce false positives or negatives.
	AssuranceHeuristic Assurance = "heuristic"
	// AssuranceAdvisory communicates guidance without automated truth
	// judgment.
	AssuranceAdvisory Assurance = "advisory"
)

// Valid reports whether the value is a defined enum member.
func (a Assurance) Valid() bool {
	switch a {
	case AssuranceExact, AssurancePartial, AssuranceHeuristic, AssuranceAdvisory:
		return true
	}
	return false
}

// PermitsConformance decides whether absence of a Violation can justify
// the conforms outcome. Only exact evidence decides the Claim; partial
// evidence leaves absence undetermined.
func (a Assurance) PermitsConformance() bool { return a == AssuranceExact }

// EvidenceMethod names how Enforcement obtained the evidence used for
// an evaluation, without overstating semantic knowledge.
type EvidenceMethod string

// Describe explains how ArcLint reached its conclusion.
func (e EvidenceMethod) Describe() string { return string(e) }

// Enforcement describes how one Rule is evaluated: declared language
// coverage, required facts, evidence, assurance, and limitations. A
// Rule whose Enforcement cannot evaluate yields the unsupported
// outcome, never implicit conformance.
type Enforcement struct {
	// languages is nil for language-independent enforcement operating on
	// path facts alone.
	languages   []Language
	facts       []Fact
	evidence    EvidenceMethod
	assurance   Assurance
	limitations []string
	implemented bool
}

// NewEnforcement requires fact requirements, a named evidence method,
// and a defined assurance.
func NewEnforcement(languages []Language, facts []Fact, evidence EvidenceMethod,
	assurance Assurance, limitations []string, implemented bool,
) (Enforcement, error) {
	if len(facts) == 0 {
		return Enforcement{}, fmt.Errorf("enforcement: no fact requirements declared")
	}
	if strings.TrimSpace(string(evidence)) == "" {
		return Enforcement{}, fmt.Errorf("enforcement: empty evidence method")
	}
	if !assurance.Valid() {
		return Enforcement{}, fmt.Errorf("enforcement: assurance %q invalid", assurance)
	}
	for _, l := range languages {
		if !l.Valid() {
			return Enforcement{}, fmt.Errorf("enforcement: language %q invalid", l)
		}
	}
	return Enforcement{
		languages:   append([]Language(nil), languages...),
		facts:       append([]Fact(nil), facts...),
		evidence:    evidence,
		assurance:   assurance,
		limitations: append([]string(nil), limitations...),
		implemented: implemented,
	}, nil
}

// Languages returns declared language coverage; nil means
// language-independent.
func (e Enforcement) Languages() []Language { return append([]Language(nil), e.languages...) }

// Facts returns the required fact classes.
func (e Enforcement) Facts() []Fact { return append([]Fact(nil), e.facts...) }

// Evidence returns the declared evidence method.
func (e Enforcement) Evidence() EvidenceMethod { return e.evidence }

// Assurance returns the declared conclusion strength.
func (e Enforcement) Assurance() Assurance { return e.assurance }

// Limitations returns the documented analysis limits.
func (e Enforcement) Limitations() []string { return append([]string(nil), e.limitations...) }

// CanEvaluate reports whether this build of ArcLint can perform the
// enforcement. When false, evaluation yields unsupported.
func (e Enforcement) CanEvaluate() bool { return e.implemented }

// SupportsLanguage determines compatibility with one language.
// Language-independent enforcement supports every file.
func (e Enforcement) SupportsLanguage(l Language) bool {
	if e.languages == nil {
		return true
	}
	for _, known := range e.languages {
		if known == l {
			return true
		}
	}
	return false
}

// IsZero reports an unconstructed Enforcement.
func (e Enforcement) IsZero() bool { return e.evidence == "" }

// BuiltinEnforcement is the canonical Enforcement of each Rule Type in
// this build of the target engine. Import-fact Rules cover Go only for
// now; expression Rules are declared but not yet evaluable, so they
// honestly yield unsupported.
func BuiltinEnforcement(t Type) Enforcement {
	switch t {
	case TypeConsumes, TypeLayers, TypeProtected, TypeIndependence, TypeAcyclic:
		e, _ := NewEnforcement([]Language{LanguageGo, LanguageTypeScript, LanguagePython},
			[]Fact{FactImports},
			"static import classification against the module and dependency manifests",
			AssuranceExact, nil, true)
		return e
	case TypeStructure:
		e, _ := NewEnforcement(nil, []Fact{FactFileTree},
			"repository file tree matching", AssuranceExact, nil, true)
		return e
	case TypeNaming:
		e, _ := NewEnforcement(nil, []Fact{FactFileTree},
			"file name matching against the case vocabulary", AssuranceExact, nil, true)
		return e
	case TypeInvariants:
		e, _ := NewEnforcement([]Language{LanguageGo, LanguageTypeScript, LanguagePython},
			[]Fact{FactDeclarations, FactCalls},
			"declaration and call matching against recorded domain contracts",
			AssuranceExact, nil, true)
		return e
	case TypeContent:
		e, _ := NewEnforcement(nil, []Fact{FactFileTree},
			"line matching against the forbidden pattern over file bytes", AssuranceExact, nil, true)
		return e
	case TypeExtension:
		e, _ := NewEnforcement(nil, []Fact{FactFileTree, FactImports, FactDeclarations},
			"TypeScript extension executed in the sandboxed SDK runtime",
			AssuranceHeuristic,
			[]string{"the engine treats extension evidence as heuristic regardless of the extension's own declaration"}, true)
		return e
	}
	return Enforcement{}
}
