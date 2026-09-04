package rule

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// Expansion is a value object a structure Rule may carry: its glob
// parameters derive from a recorded Ubiquitous Language collection,
// one substitution per recorded term. The Rule stays the one and only
// aggregate: an expanded Rule is one Rule stating one universally
// quantified claim; Expansion only describes how its globs are
// derived. Derivation is pure substitution over a closed placeholder
// grammar: no predicates, no filters, no joins; logic beyond
// substitution belongs to the Extension SDK. A project recording
// nothing derives empty parameters: the Rule exists, asserts nothing
// yet, and says so in its Claim.
type Expansion struct {
	source  ExpansionSource
	require []string
	forbid  []string
}

// ExpansionSource names one recorded collection an Expansion derives
// from. The set is closed; a new source is a deliberate design act,
// never a syntax feature.
type ExpansionSource string

// The published Expansion sources.
const (
	SourceAggregates     ExpansionSource = "domain.aggregates"
	SourceEntities       ExpansionSource = "domain.entities"
	SourceValueObjects   ExpansionSource = "domain.value_objects"
	SourceEvents         ExpansionSource = "domain.events"
	SourceContexts       ExpansionSource = "domain.contexts"
	SourceInvariants     ExpansionSource = "domain.invariants"
	SourceAssertions     ExpansionSource = "domain.assertions"
	SourceSpecifications ExpansionSource = "domain.specifications"
)

// ExpansionSources returns the published sources in declaration order.
func ExpansionSources() []ExpansionSource {
	return []ExpansionSource{
		SourceAggregates, SourceEntities, SourceValueObjects,
		SourceEvents, SourceContexts,
		SourceInvariants, SourceAssertions, SourceSpecifications,
	}
}

// Valid reports whether the value is a published source.
func (s ExpansionSource) Valid() bool {
	return slices.Contains(ExpansionSources(), s)
}

// placeholderPattern matches one placeholder occurrence; the case name
// is validated against the published term cases, not by this pattern.
var placeholderPattern = regexp.MustCompile(`\{name:([^{}]*)\}`)

// probeTerm stands in for a recorded term during construction-time
// validation, so an invalid Expansion never loads.
const probeTerm = "Sample Term"

// NewExpansion constructs a valid Expansion or rejects it completely:
// an unknown source, a malformed placeholder, an unknown term case,
// and a glob that cannot become valid after substitution are
// construction errors.
func NewExpansion(source string, require, forbid []string) (Expansion, error) {
	s := ExpansionSource(source)
	if !s.Valid() {
		names := make([]string, 0, len(ExpansionSources()))
		for _, known := range ExpansionSources() {
			names = append(names, string(known))
		}
		return Expansion{}, fmt.Errorf("each %q: not one of %s", source, strings.Join(names, ", "))
	}
	if len(require)+len(forbid) == 0 {
		return Expansion{}, fmt.Errorf("each %s: at least one require or forbid glob must be declared", s)
	}
	e := Expansion{
		source:  s,
		require: append([]string(nil), require...),
		forbid:  append([]string(nil), forbid...),
	}
	for _, list := range [][]string{e.require, e.forbid} {
		for _, g := range list {
			if err := validateExpandedGlob(g); err != nil {
				return Expansion{}, err
			}
			// The probe substitution proves the glob is valid once a
			// real term resolves it.
			resolved, err := substitutePlaceholders(g, probeTerm)
			if err != nil {
				return Expansion{}, err
			}
			if _, err := NewGlob(resolved); err != nil {
				return Expansion{}, fmt.Errorf("glob %q: %v", g, err)
			}
		}
	}
	return e, nil
}

// validateExpandedGlob rejects any brace region that is not a
// well-formed {name:<published case>} placeholder.
func validateExpandedGlob(glob string) error {
	stripped := placeholderPattern.ReplaceAllStringFunc(glob, func(m string) string {
		caseName := placeholderPattern.FindStringSubmatch(m)[1]
		if _, err := CaseTerm(probeTerm, caseName); err != nil {
			return m // leave invalid case names for the brace check below
		}
		return ""
	})
	if strings.ContainsAny(stripped, "{}") {
		return fmt.Errorf("glob %q: braces are valid only as {name:<case>} placeholders with a case among %s",
			glob, strings.Join(TermCaseNames(), ", "))
	}
	return nil
}

// Source returns the recorded collection the Expansion derives from.
func (e Expansion) Source() ExpansionSource { return e.source }

// Require returns the authored require globs, placeholders unresolved.
func (e Expansion) Require() []string { return append([]string(nil), e.require...) }

// Forbid returns the authored forbid globs, placeholders unresolved.
func (e Expansion) Forbid() []string { return append([]string(nil), e.forbid...) }

// IsZero reports an unconstructed Expansion.
func (e Expansion) IsZero() bool { return e.source == "" }

// Resolve derives the structure parameters from a recorded language:
// every authored glob substituted once per recorded term of the
// source, duplicates collapsed (a glob without placeholders resolves
// identically for every term and is one obligation, not many). An
// empty vocabulary resolves to empty parameters.
func (e Expansion) Resolve(lang vocab.UbiquitousLanguage) (StructureParams, error) {
	resolve := func(globs []string) ([]Glob, error) {
		var resolved []string
		seen := map[string]bool{}
		for _, term := range sourceTerms(e.source, lang) {
			for _, g := range globs {
				r, err := substitutePlaceholders(g, term)
				if err != nil {
					return nil, fmt.Errorf("term %q: %v", term, err)
				}
				if !seen[r] {
					seen[r] = true
					resolved = append(resolved, r)
				}
			}
		}
		return NewGlobs(resolved)
	}
	require, err := resolve(e.require)
	if err != nil {
		return StructureParams{}, fmt.Errorf("each %s: %v", e.source, err)
	}
	forbid, err := resolve(e.forbid)
	if err != nil {
		return StructureParams{}, fmt.Errorf("each %s: %v", e.source, err)
	}
	return StructureParams{Require: require, Forbid: forbid}, nil
}

// substitutePlaceholders resolves every {name:<case>} occurrence for
// one term.
func substitutePlaceholders(glob, term string) (string, error) {
	var substErr error
	out := placeholderPattern.ReplaceAllStringFunc(glob, func(m string) string {
		caseName := placeholderPattern.FindStringSubmatch(m)[1]
		segment, err := CaseTerm(term, caseName)
		if err != nil && substErr == nil {
			substErr = fmt.Errorf("glob %q: %v", glob, err)
		}
		return segment
	})
	if substErr != nil {
		return "", substErr
	}
	return out, nil
}

// sourceTerms projects the recorded language onto one source's terms,
// in file order.
func sourceTerms(source ExpansionSource, lang vocab.UbiquitousLanguage) []string {
	var out []string
	for _, c := range lang.Contexts {
		switch source {
		case SourceContexts:
			out = append(out, c.Name)
		case SourceAggregates:
			for _, e := range c.Entities {
				if e.Aggregate {
					out = append(out, e.Name)
				}
			}
		case SourceEntities:
			for _, e := range c.Entities {
				out = append(out, e.Name)
			}
		case SourceValueObjects:
			for _, d := range c.ValueObjects {
				out = append(out, d.Name)
			}
		case SourceEvents:
			for _, d := range c.Events {
				out = append(out, d.Name)
			}
		case SourceInvariants:
			for _, inv := range c.Invariants {
				if inv.ID != "" {
					out = append(out, inv.ID)
					continue
				}
				out = append(out, inv.Owner)
			}
		case SourceAssertions:
			for _, a := range c.Assertions {
				out = append(out, a.ID)
			}
		case SourceSpecifications:
			for _, s := range c.Specifications {
				out = append(out, s.Name)
			}
		}
	}
	return out
}
