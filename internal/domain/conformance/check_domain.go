package conformance

import (
	"fmt"
	"path"
	"sort"
	"strings"
	"unicode"

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

// The words a contract's method begins with: an aggregate invariant is
// enforced by ensure followed by its key (EnsureLinesFrozen), an
// assertion is checked by assert followed by its key (AssertTiersPriced).
const (
	ensurePrefix = "ensure-"
	assertPrefix = "assert-"
)

// ensureKey is the recorded key of the method enforcing an invariant.
func ensureKey(key string) string { return ensurePrefix + key }

// assertKey is the recorded key of the method checking an assertion.
func assertKey(key string) string { return assertPrefix + key }

// domainCheck judges one block invariant of the Domain-Driven Design
// meta-model over one bounded context. A check that needs Zones judges
// an outside the ruleset declares; without a declared Zone it does not
// apply.
type domainCheck struct {
	run        func(r rule.Rule, subject rule.Subject, cc contextCode, code domainCode) ([]Violation, error)
	needsZones bool
}

// domainChecks maps every block invariant the domain evaluator carries
// to its check, by the invariant's id in the meta-model. A check-level
// invariant without an entry fails evaluation loudly, never silently
// conforms.
var domainChecks = map[string]domainCheck{
	"ubiquitous_language/terms-declared-in-code":            {run: checkTermsDeclared},
	"bounded_context/code-held-by-one-context":              {run: checkCodeHeldByOneContext},
	"bounded_context/isolated":                              {run: checkContextIsolated},
	"context_relation/imports-follow-influence":             {run: checkImportsFollowInfluence},
	"domain_isolation/model-imports-nothing-outside-itself": {run: checkModelImportsNothingOutside, needsZones: true},
	"value_object/constructed-through-one-door":             {run: checkValueObjectConstructor},
	"value_object/no-setters":                               {run: checkValueObjectNoSetters},
	"invariant/enforced-at-every-mutation":                  {run: checkInvariantEnforcedAtMutation},
	"assertion/checked-by-its-operation":                    {run: checkAssertionChecked},
	"specification/satisfaction-method":                     {run: checkSpecificationSatisfaction},
	"aggregate/protects-an-invariant":                       {run: checkAggregateProtectsInvariant},
	"aggregate/root-declared":                               {run: checkAggregateRootDeclared},
	"aggregate/invariants-enforced-by-root":                 {run: checkAggregateInvariantsOnRoot},
	"aggregate/commands-named-for-behavior":                 {run: checkAggregateNoSetters},
	"aggregate/references-by-identity":                      {run: checkReferencesByIdentity},
	"domain_event/declared":                                 {run: checkEventDeclared},
	"domain_event/no-setters":                               {run: checkEventNoSetters},
	"domain_service/declared":                               {run: checkServiceDeclared},
	"repository/declared":                                   {run: checkRepositoryDeclared},
	"factory/declared":                                      {run: checkFactoryDeclared},
}

// evaluateDomain judges one built-in domain Rule: the block invariant
// its params name, over every bounded context the recorded domain
// holds. Each context is one Subject, judged through the code located
// for it; a context whose scope yields none of the facts the invariant
// reads is unsupported, and a check that needs Zones is not applicable
// while the ruleset declares none.
func evaluateDomain(r rule.Rule, mem membership, obs Observations, knowledge vocab.UbiquitousLanguage) ([]Evaluation, error) {
	p, ok := r.Params().(rule.DomainParams)
	if !ok {
		return nil, fmt.Errorf("rule %s: domain rule with %T params", r.ID(), r.Params())
	}
	inv, err := p.BlockInvariant()
	if err != nil {
		return nil, fmt.Errorf("rule %s: %w", r.ID(), err)
	}
	check, ok := domainChecks[inv.ID]
	if !ok {
		return nil, fmt.Errorf("rule %s: no evaluator for block invariant %s", r.ID(), inv.ID)
	}
	code, err := resolveDomain(r.Scope().ExcludedFile, mem, obs, knowledge)
	if err != nil {
		return nil, fmt.Errorf("rule %s: %w", r.ID(), err)
	}
	var out []Evaluation
	for _, cc := range code.contexts {
		subject, err := rule.ContextSubject(cc.ctx.Name)
		if err != nil {
			return nil, fmt.Errorf("domain: %w", err)
		}
		outcome, undecided := cc.undecidable(inv.Enforcement.Facts)
		if check.needsZones && len(mem.names) == 0 {
			outcome, undecided = OutcomeNotApplicable, true
		}
		if undecided {
			e, err := simpleEvaluation(r, subject, outcome)
			if err != nil {
				return nil, err
			}
			out = append(out, e)
			continue
		}
		vs, err := check.run(r, subject, cc, code)
		if err != nil {
			return nil, fmt.Errorf("rule %s: %w", r.ID(), err)
		}
		e, err := completeEvaluation(r, subject, vs)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

// domainCode is the recorded domain located in the code: each context
// with the code found for it, which contexts' code each file is, and
// every aggregate root located anywhere in the domain.
type domainCode struct {
	knowledge vocab.UbiquitousLanguage
	contexts  []contextCode
	holders   map[string][]string
	roots     []aggregateRoot
}

// aggregateRoot is one located root with the aggregate and context it
// belongs to and the unit holding it.
type aggregateRoot struct {
	context string
	agg     vocab.Aggregate
	root    locDecl
	unit    string
}

// contextCode is one bounded context with the code located for it.
//
// The scope is where the context's declarations are looked for: the
// member files of the Zone spelled with the context's name when the
// ruleset declares one, otherwise every observed file; an Exclusion of
// the Rule removes a file from it. Nothing in the domain file names a
// path.
//
// Each aggregate's root is the one type in scope spelling its name, and
// the aggregate's unit is the directory holding the root: the root's
// methods, constructors, and commands are read there. Every other
// recorded term is looked for inside the units first and then in the
// rest of the scope. The context's code is every file of the Zone
// named for it when the ruleset declares one, since naming a Zone for
// a context says the Zone is that context; otherwise it is the scope
// files of its units plus the file of every term located outside them.
type contextCode struct {
	ctx      vocab.BoundedContext
	zone     rule.ZoneName
	scope    []string
	scopeIdx contractIndex
	// scopeImportFiles counts the scope files yielding the imports fact.
	scopeImportFiles int
	aggregates       map[string]aggregateCode
	terms            map[string]termLocation
	files            []string
	idx              contractIndex
	imports          []fileImport
	importFiles      int
}

// aggregateCode is one aggregate's root as located: exactly one
// candidate when it is found, none when nothing spells it, several
// when nothing tells which is the model's. A located aggregate has a
// unit (the directory of the root) and the facts of that unit.
type aggregateCode struct {
	candidates []locDecl
	unit       string
	idx        contractIndex
}

func (a aggregateCode) located() bool { return len(a.candidates) == 1 }

func (a aggregateCode) root() locDecl { return a.candidates[0] }

// termLocation is one recorded term as located: exactly one candidate
// when it is found, none when nothing spells it, several when nothing
// tells which is the model's.
type termLocation struct {
	candidates []locDecl
}

func (t termLocation) located() bool { return len(t.candidates) == 1 }

func (t termLocation) ambiguous() bool { return len(t.candidates) > 1 }

func (t termLocation) decl() locDecl { return t.candidates[0] }

// fileImport is one internal import of a context file with the
// declared Zones it lands in.
type fileImport struct {
	path    string
	imp     Import
	targets []rule.ZoneName
}

// resolveDomain locates every context of the recorded domain in the
// observed code. The excluded function removes a file from every
// scope; the Rule's Exclusions when a Rule is judged, nothing when the
// domain is listed.
func resolveDomain(excluded func(string) bool, mem membership, obs Observations, knowledge vocab.UbiquitousLanguage) (domainCode, error) {
	code := domainCode{knowledge: knowledge, holders: map[string][]string{}}
	for _, ctx := range knowledge.Contexts {
		cc, err := resolveContext(ctx, mem, obs, excluded)
		if err != nil {
			return domainCode{}, err
		}
		for _, f := range cc.files {
			code.holders[f] = append(code.holders[f], ctx.Name)
		}
		for _, agg := range ctx.Aggregates {
			ac := cc.aggregates[agg.Name]
			if ac.located() {
				code.roots = append(code.roots, aggregateRoot{context: ctx.Name, agg: agg, root: ac.root(), unit: ac.unit})
			}
		}
		code.contexts = append(code.contexts, cc)
	}
	for f := range code.holders {
		sort.Strings(code.holders[f])
	}
	return code, nil
}

func resolveContext(ctx vocab.BoundedContext, mem membership, obs Observations, excluded func(string) bool) (contextCode, error) {
	cc := contextCode{ctx: ctx, aggregates: map[string]aggregateCode{}, terms: map[string]termLocation{}}
	zone, scope, err := contextScope(ctx.Name, mem, excluded)
	if err != nil {
		return contextCode{}, err
	}
	cc.zone, cc.scope = zone, scope
	cc.scopeIdx = buildContractIndex(scope, obs)
	cc.scopeImportFiles = importSupport(scope, obs)
	seen := map[string]bool{}
	add := func(f string) {
		if !seen[f] {
			seen[f] = true
			cc.files = append(cc.files, f)
		}
	}
	if zone != "" {
		for _, f := range scope {
			add(f)
		}
	}
	for _, agg := range ctx.Aggregates {
		ac := cc.locateRoot(agg)
		cc.aggregates[agg.Name] = ac
		if !ac.located() {
			continue
		}
		for _, f := range scope {
			if path.Dir(f) == ac.unit {
				add(f)
			}
		}
	}
	for _, name := range cc.termNames() {
		loc := cc.locate(name, false)
		cc.terms[name] = loc
		if loc.located() {
			add(loc.decl().file.path)
		}
	}
	for _, agg := range ctx.Aggregates {
		if agg.Factory == "" {
			continue
		}
		loc := cc.locate(agg.Factory, true)
		cc.terms[agg.Factory] = loc
		if loc.located() {
			add(loc.decl().file.path)
		}
	}
	sort.Strings(cc.files)
	cc.idx = buildContractIndex(cc.files, obs)
	cc.imports, cc.importFiles = contextImports(cc.files, mem, obs)
	return cc, nil
}

// contextScope is where a context's declarations are looked for: the
// member files of the Zone named for the context when the ruleset
// declares one (its exact name first, then the same words in another
// case), otherwise every observed file. Two Zones spelling the name in
// different cases leave nothing to choose between, and that is an
// error, never a silent pick.
func contextScope(name string, mem membership, excluded func(string) bool) (rule.ZoneName, []string, error) {
	keep := func(files []string) []string {
		out := make([]string, 0, len(files))
		for _, f := range files {
			if !excluded(f) {
				out = append(out, f)
			}
		}
		sort.Strings(out)
		return out
	}
	if _, ok := mem.zones[rule.ZoneName(name)]; ok {
		return rule.ZoneName(name), keep(mem.zoneFiles[rule.ZoneName(name)]), nil
	}
	var spelled []rule.ZoneName
	for _, z := range mem.names {
		if namedFor(string(z), name) {
			spelled = append(spelled, z)
		}
	}
	switch len(spelled) {
	case 0:
		return "", keep(mem.files), nil
	case 1:
		return spelled[0], keep(mem.zoneFiles[spelled[0]]), nil
	default:
		return "", nil, fmt.Errorf("context %s: Zones %s all spell its name; keep one named for the context", name, quotedZones(spelled))
	}
}

// importSupport counts the files yielding the imports fact.
func importSupport(files []string, obs Observations) int {
	n := 0
	for _, f := range files {
		if facts, ok := obs.FactsFor(f); ok && facts.Supports(rule.FactImports) {
			n++
		}
	}
	return n
}

// termNames lists every recorded term of the context looked for as a
// type: member entities, value objects, identities, events, services,
// specifications, and the repositories its aggregates name. Aggregates
// are located as roots, factories as types or functions.
func (cc contextCode) termNames() []string {
	var out []string
	for _, t := range cc.ctx.Terms() {
		if t.Concept == vocab.ConceptAggregate {
			continue
		}
		out = append(out, t.Name)
	}
	for _, agg := range cc.ctx.Aggregates {
		if agg.Repository != "" {
			out = append(out, agg.Repository)
		}
	}
	return out
}

// locateRoot finds an aggregate's root in the scope: the type
// declarations spelling its name, narrowed to those that can carry
// behaviour (a struct or named type in Go, a class in TypeScript and
// Python) when any does, so an interface or alias of the same name
// never competes with the type that holds the commands.
func (cc contextCode) locateRoot(agg vocab.Aggregate) aggregateCode {
	all := cc.scopeIdx.declsNamed(agg.Name, false)
	var strong []locDecl
	for _, d := range all {
		if carriesBehaviour(d) {
			strong = append(strong, d)
		}
	}
	if len(strong) > 0 {
		all = strong
	}
	ac := aggregateCode{candidates: all}
	if len(all) == 1 {
		ac.unit = path.Dir(all[0].file.path)
		ac.idx = cc.scopeIdx.within(ac.unit)
	}
	return ac
}

// carriesBehaviour reports a type declaration that can hold methods in
// its language, the shape an aggregate root takes.
func carriesBehaviour(d locDecl) bool {
	switch d.file.lang {
	case rule.LanguageGo:
		return d.decl.Kind == declKindStruct || d.decl.Kind == declKindType
	case rule.LanguageTypeScript, rule.LanguagePython:
		return d.decl.Kind == declKindClass
	default:
		return d.decl.Kind == declKindStruct || d.decl.Kind == declKindClass
	}
}

// locate finds a recorded term: inside the located aggregate units
// first, where the context's code is settled, and only then in the
// rest of the scope. Every candidate is kept, so a term two
// declarations spell is reported, never picked.
func (cc contextCode) locate(name string, allowFunc bool) termLocation {
	var inUnits []locDecl
	for _, agg := range cc.ctx.Aggregates {
		ac := cc.aggregates[agg.Name]
		if ac.located() {
			inUnits = append(inUnits, ac.idx.declsNamed(name, allowFunc)...)
		}
	}
	if found := uniqueDecls(inUnits); len(found) > 0 {
		return termLocation{candidates: found}
	}
	return termLocation{candidates: cc.scopeIdx.declsNamed(name, allowFunc)}
}

// uniqueDecls drops the duplicates two aggregates sharing one unit
// produce, keeping first appearances in order.
func uniqueDecls(ds []locDecl) []locDecl {
	type key struct {
		path string
		line int
		name string
	}
	seen := map[key]bool{}
	var out []locDecl
	for _, d := range ds {
		k := key{d.file.path, d.decl.StartLine, d.decl.Name}
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, d)
	}
	return out
}

// unitIdx is the facts of the unit holding a declaration: the files of
// its directory, where its methods and constructors are read.
func (cc contextCode) unitIdx(d locDecl) contractIndex {
	return cc.scopeIdx.within(path.Dir(d.file.path))
}

// undecidable reports the outcome of a context the invariant's facts
// cannot judge: unsupported when the scope has files but none yields a
// fact the invariant reads. An empty scope is decidable: nothing in it
// declares the recorded terms, and the checks say so.
func (cc contextCode) undecidable(facts []string) (Outcome, bool) {
	if len(facts) == 0 || len(cc.scope) == 0 {
		return "", false
	}
	for _, f := range facts {
		switch rule.Fact(f) {
		case rule.FactDeclarations, rule.FactCalls:
			if len(cc.scopeIdx.files) == 0 {
				return OutcomeUnsupported, true
			}
		case rule.FactImports:
			if cc.scopeImportFiles == 0 {
				return OutcomeUnsupported, true
			}
		case rule.FactFileTree:
			// The walk that reached this context already supplied it.
		}
	}
	return "", false
}

// searched names where a context's declarations were looked for.
func (cc contextCode) searched() string {
	if cc.zone != "" {
		return fmt.Sprintf("Zone %q", cc.zone)
	}
	return "the repository"
}

// narrowRemedy says how to leave one declaration for a name two spell.
func (cc contextCode) narrowRemedy() string {
	if cc.zone != "" {
		return fmt.Sprintf("narrow Zone %q to the context's code, or exclude the twin under scan.exclude in rules.arclint.yaml", cc.zone)
	}
	return fmt.Sprintf("declare a Zone %q over the context's code in rules.arclint.yaml, or exclude the twin under scan.exclude", cc.ctx.Name)
}

// locationViolations reports a recorded name that nothing declares, or
// that several declarations spell, at its record in the domain file. A
// located name yields nothing.
func (cc contextCode) locationViolations(r rule.Rule, subject rule.Subject, what, name string, line int, loc termLocation, declared, remedy string) ([]Violation, error) {
	if loc.located() {
		return nil, nil
	}
	file, anchor := recordAnchor(line)
	if loc.ambiguous() {
		v, err := newViolation(r, subject, file, anchor,
			fmt.Sprintf("%s %s of context %s is declared %d times in %s (%s), and nothing tells which is the model's", what, name, cc.ctx.Name, len(loc.candidates), cc.searched(), candidateList(loc.candidates)),
			cc.narrowRemedy())
		if err != nil {
			return nil, err
		}
		return []Violation{v}, nil
	}
	v, err := newViolation(r, subject, file, anchor,
		fmt.Sprintf("%s %s of context %s names no %s declaration in %s", what, name, cc.ctx.Name, declared, cc.searched()),
		remedy)
	if err != nil {
		return nil, err
	}
	return []Violation{v}, nil
}

// candidateList spells declarations as path:line, comma separated.
func candidateList(ds []locDecl) string {
	parts := make([]string, 0, len(ds))
	for _, d := range ds {
		parts = append(parts, fmt.Sprintf("%s:%d", d.file.path, d.decl.StartLine))
	}
	return strings.Join(parts, ", ")
}

// contextsOf is the sorted set of contexts whose code an import lands
// in: the contexts holding the target file, or, for an import resolved
// to a directory alone, the contexts holding any file beneath it.
func (code domainCode) contextsOf(imp Import) []string {
	if imp.TargetFile != "" {
		return code.holders[imp.TargetFile]
	}
	if imp.TargetDir == "" {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for f, names := range code.holders {
		if !under(f, imp.TargetDir) {
			continue
		}
		for _, name := range names {
			if !seen[name] {
				seen[name] = true
				out = append(out, name)
			}
		}
	}
	sort.Strings(out)
	return out
}

// contextImports collects the internal imports of the files that yield
// the imports fact, with the count of files that yield it.
func contextImports(files []string, mem membership, obs Observations) ([]fileImport, int) {
	var out []fileImport
	supporting := 0
	for _, f := range files {
		facts, ok := obs.FactsFor(f)
		if !ok || !facts.Supports(rule.FactImports) {
			continue
		}
		supporting++
		for _, imp := range facts.Imports {
			if imp.Class != ImportInternal {
				continue
			}
			out = append(out, fileImport{path: f, imp: imp, targets: mem.targetZones(imp)})
		}
	}
	return out, supporting
}

// importsUnit reports whether a file of the context imports a unit: a
// target file inside the directory, or the directory itself.
func (cc contextCode) importsUnit(file, unit string) bool {
	for _, fi := range cc.imports {
		if fi.path != file {
			continue
		}
		if fi.imp.TargetDir == unit || (fi.imp.TargetFile != "" && path.Dir(fi.imp.TargetFile) == unit) {
			return true
		}
	}
	return false
}

func under(p, dir string) bool {
	return strings.HasPrefix(p, dir+"/")
}

// namedFor reports whether a path segment or a file stem spells a
// recorded term: the same words in any published case, so order_line,
// order-line, orderline, OrderLine, and orderLine all spell OrderLine.
func namedFor(segment, term string) bool {
	s, err := rule.CaseTerm(segment, "flatcase")
	if err != nil {
		return false
	}
	t, err := rule.CaseTerm(term, "flatcase")
	if err != nil {
		return false
	}
	return s == t
}

// typeSpelled reports whether a declared name spells a recorded term in
// type case: the same words, starting with a capital and carrying no
// separator, so EventID spells EventID and event_id does not.
func typeSpelled(declared, term string) bool {
	if declared == "" || strings.ContainsAny(declared, "_-") {
		return false
	}
	if !unicode.IsUpper([]rune(declared)[0]) {
		return false
	}
	return namedFor(declared, term)
}

// methodCase is the case a language spells a method or function in.
func methodCase(lang rule.Language) string {
	switch lang {
	case rule.LanguageTypeScript:
		return "camelCase"
	case rule.LanguagePython:
		return "snake_case"
	default:
		return "PascalCase"
	}
}

// methodNameFor spells a recorded key or operation as the method of
// one language.
func methodNameFor(term string, lang rule.Language) (string, error) {
	name, err := rule.CaseTerm(term, methodCase(lang))
	if err != nil {
		return "", fmt.Errorf("method name for %q: %w", term, err)
	}
	return name, nil
}

// methodSpelled reports whether a declared method name spells a
// recorded key in the method case of one language: the same words, in
// the shape that language spells a method (PascalCase in Go, camelCase
// in TypeScript, snake_case in Python). Initialisms keep their own
// capitals, so UniqueQualifiedID and UniqueQualifiedId both spell
// unique-qualified-id in Go, while publishedFrozen spells nothing there.
func methodSpelled(declared, key string, lang rule.Language) bool {
	if declared == "" || !namedFor(declared, key) {
		return false
	}
	first := []rune(declared)[0]
	switch lang {
	case rule.LanguagePython:
		return strings.ToLower(declared) == declared
	case rule.LanguageTypeScript:
		return unicode.IsLower(first) && !strings.ContainsAny(declared, "_-")
	default:
		return unicode.IsUpper(first) && !strings.ContainsAny(declared, "_-")
	}
}

// methodSpellings lists the method a key names in every language the
// indexed files are written in, for a finding to say what it expected.
func methodSpellings(idx contractIndex, term string) string {
	var parts []string
	seen := map[rule.Language]bool{}
	for _, f := range idx.files {
		if seen[f.lang] {
			continue
		}
		seen[f.lang] = true
		name, err := methodNameFor(term, f.lang)
		if err != nil {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s (%s)", name, f.lang))
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}

// recordAnchor anchors a finding about a recorded instance at its line
// in the domain file.
func recordAnchor(line int) (string, int) {
	return vocab.UbiquitousLanguageFileName, line
}

// fileFacts is one parsed file of a context: its language, package,
// declarations, and calls.
type fileFacts struct {
	path  string
	lang  rule.Language
	pkg   string
	decls []Declaration
	calls []Call
}

// contractIndex is the parsed facts of a set of files.
type contractIndex struct {
	files []fileFacts
}

func buildContractIndex(paths []string, obs Observations) contractIndex {
	var files []fileFacts
	for _, p := range paths {
		facts, ok := obs.FactsFor(p)
		if !ok || facts.ParseFailure != "" {
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

// within narrows the index to the files of one directory: a Go package,
// or the module files beside a TypeScript or Python declaration.
func (idx contractIndex) within(dir string) contractIndex {
	var files []fileFacts
	for _, f := range idx.files {
		if path.Dir(f.path) == dir {
			files = append(files, f)
		}
	}
	return contractIndex{files: files}
}

// locDecl is one declaration with the file it is in.
type locDecl struct {
	file fileFacts
	decl Declaration
}

// declsNamed lists every type declaration spelling a term and, when
// functions are allowed, every function spelling it in the file's
// method case or with the recorded spelling.
func (idx contractIndex) declsNamed(term string, allowFunc bool) []locDecl {
	var out []locDecl
	for _, f := range idx.files {
		for _, d := range f.decls {
			switch {
			case isTypeKind(d.Kind) && f.spellsType(d.Name, term):
			case allowFunc && d.Kind == declKindFunc && (d.Name == term || methodSpelled(d.Name, term, f.lang)):
			default:
				continue
			}
			out = append(out, locDecl{file: f, decl: d})
		}
	}
	return out
}

// spellsType reports whether a declared name of the file spells a
// recorded term in type case. In Go the package's own name completes
// the declared name, since a type is not named after its package:
// rule.ID spells RuleID, and order.Line spells OrderLine.
func (f fileFacts) spellsType(declared, term string) bool {
	if typeSpelled(declared, term) {
		return true
	}
	if f.lang != rule.LanguageGo || f.pkg == "" {
		return false
	}
	return typeSpelled(declared, declared) && namedFor(f.pkg+"_"+declared, term)
}

func isTypeKind(kind string) bool {
	switch kind {
	case declKindStruct, declKindClass, declKindType, declKindInterface:
		return true
	}
	return false
}

// methodsOn lists the methods declared on a type across the index.
func (idx contractIndex) methodsOn(owner string) []locDecl {
	var out []locDecl
	for _, f := range idx.files {
		for _, d := range f.decls {
			if d.Kind == declKindMethod && d.Owner == owner {
				out = append(out, locDecl{file: f, decl: d})
			}
		}
	}
	return out
}

// methodNamed finds the method a key names on a type, spelled in the
// method case of the file's language. A key no case can spell is an
// error, never a silent miss.
func (idx contractIndex) methodNamed(owner, key string) (locDecl, bool, error) {
	if _, err := rule.CaseTerm(key, "flatcase"); err != nil {
		return locDecl{}, false, fmt.Errorf("method name for %q: %w", key, err)
	}
	for _, f := range idx.files {
		for _, d := range f.decls {
			if d.Kind == declKindMethod && d.Owner == owner && methodSpelled(d.Name, key, f.lang) {
				return locDecl{file: f, decl: d}, true, nil
			}
		}
	}
	return locDecl{}, false, nil
}

// constructors lists the doors a type is built through: a constructor
// or __init__ on it, a factory method on it named create, from, of,
// parse, new, or build that returns it, and every function of the unit
// that returns it. A function returning a collection of the type is
// not a door; the values inside it were built elsewhere.
func (idx contractIndex) constructors(typeName string) []locDecl {
	var out []locDecl
	for _, f := range idx.files {
		if isTestFile(f.path) {
			continue
		}
		for _, d := range f.decls {
			if isConstructor(d, typeName) {
				out = append(out, locDecl{file: f, decl: d})
			}
		}
	}
	return out
}

// classConstructor and pyInit are the constructor methods of a class
// in TypeScript and Python.
const (
	classConstructor = "constructor"
	pyInit           = "__init__"
)

// isTestFile reports whether a path is a test file of its language: a
// helper there that builds or drives a type is the test's, not a door
// or a command of the model.
func isTestFile(p string) bool {
	base := path.Base(p)
	switch {
	case strings.HasSuffix(base, "_test.go"),
		strings.HasSuffix(base, ".test.ts"), strings.HasSuffix(base, ".spec.ts"),
		strings.HasSuffix(base, ".test.tsx"), strings.HasSuffix(base, ".spec.tsx"),
		strings.HasSuffix(base, ".test.js"), strings.HasSuffix(base, ".spec.js"),
		strings.HasPrefix(base, "test_") && strings.HasSuffix(base, ".py"),
		strings.HasSuffix(base, "_test.py"), base == "conftest.py":
		return true
	}
	return false
}

// constructorWords are the words a factory method on a type begins
// with when it builds one.
var constructorWords = []string{"create", "from", "of", "parse", "new", "build"}

func isConstructor(d Declaration, typeName string) bool {
	switch d.Kind {
	case declKindMethod:
		if d.Owner != typeName {
			return false
		}
		if d.Name == classConstructor || d.Name == pyInit {
			return true
		}
		return startsWithWord(d.Name, constructorWords...) && returnsType(d.Results, typeName)
	case declKindFunc:
		return returnsType(d.Results, typeName)
	}
	return false
}

// returnsType reports a result carrying the type itself: the type
// name as a whole token, in a result that is not a collection.
func returnsType(results []string, typeName string) bool {
	for _, r := range results {
		if !isCollection(r) && mentionsType(r, "", typeName) {
			return true
		}
	}
	return false
}

// collectionHeads are the generic collections of the supported
// languages; a result headed by one holds values, it does not build
// one.
var collectionHeads = map[string]bool{
	"Array": true, "ReadonlyArray": true, "Map": true, "ReadonlyMap": true, "Set": true, "ReadonlySet": true,
	"Record": true, "Iterable": true, "Iterator": true, "Generator": true, "AsyncIterable": true, "AsyncIterator": true,
	"list": true, "dict": true, "set": true, "tuple": true, "frozenset": true,
	"List": true, "Dict": true, "Tuple": true, "FrozenSet": true, "Sequence": true, "Mapping": true,
	"Collection": true, "Deque": true, "deque": true,
}

// isCollection reports a result text that is a slice, array, map, or
// generic collection in one of the supported languages.
func isCollection(text string) bool {
	t := strings.TrimSpace(text)
	if strings.HasPrefix(t, "[]") || strings.HasPrefix(t, "map[") || strings.HasPrefix(t, "...") || strings.HasSuffix(t, "[]") {
		return true
	}
	toks := identifierTokens(t)
	if len(toks) == 0 {
		return false
	}
	head := toks[0]
	if head == "readonly" && len(toks) > 1 {
		head = toks[1]
	}
	return collectionHeads[head] && (strings.Contains(t, "<") || strings.Contains(t, "["))
}

// mentionsType reports whether a type text names a type as a whole
// token: Event, *Event, and Promise<Event> all mention Event, while
// EventID does not. With a qualifier the token must follow it and a
// dot, as event.Event does.
func mentionsType(text, qualifier, name string) bool {
	toks := identifierTokens(text)
	for i, t := range toks {
		if t != name {
			continue
		}
		if qualifier == "" {
			return true
		}
		if i >= 2 && toks[i-1] == "." && toks[i-2] == qualifier {
			return true
		}
	}
	return false
}

// identifierTokens splits a type text into identifiers, keeping each
// dot as a token of its own so a qualifier can be matched to the name
// it qualifies.
func identifierTokens(text string) []string {
	var out []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			out = append(out, cur.String())
			cur.Reset()
		}
	}
	for _, r := range text {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_':
			cur.WriteRune(r)
		case r == '.':
			flush()
			out = append(out, ".")
		default:
			flush()
		}
	}
	flush()
	return out
}

// startsWithWord reports a name that begins with one of the words and
// continues with a new word: create, createOrder, create_order, and
// FromCents start with a word; created and offer do not.
func startsWithWord(name string, words ...string) bool {
	for _, w := range words {
		if len(name) < len(w) || !strings.EqualFold(name[:len(w)], w) {
			continue
		}
		if len(name) == len(w) {
			return true
		}
		next := []rune(name[len(w):])[0]
		if unicode.IsUpper(next) || next == '_' {
			return true
		}
	}
	return false
}

// commands lists the commands of a root: its exported methods whose
// result carries an error, other than its constructors and the methods
// its invariant and assertion keys name.
func (idx contractIndex) commands(agg vocab.Aggregate, owner string) []locDecl {
	var out []locDecl
	for _, f := range idx.files {
		if isTestFile(f.path) {
			continue
		}
		for _, d := range f.decls {
			if d.Kind != declKindMethod || d.Owner != owner || !d.Exported {
				continue
			}
			if isContractMethod(agg, d.Name, f.lang) || isConstructor(d, owner) {
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

// isContractMethod reports whether a method name spells the ensure
// method of one of the aggregate's invariants or the assert method of
// one of its assertions in one language.
func isContractMethod(agg vocab.Aggregate, name string, lang rule.Language) bool {
	for _, inv := range agg.Invariants {
		if methodSpelled(name, ensureKey(inv.Key), lang) {
			return true
		}
	}
	for _, as := range agg.Assertions {
		if methodSpelled(name, assertKey(as.Key), lang) {
			return true
		}
	}
	return false
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

// callsKey reports whether a declaration of the file calls a method
// spelling the key in the file's language.
func callsKey(f fileFacts, enclosing, key string) bool {
	for _, c := range f.calls {
		if c.Enclosing == enclosing && methodSpelled(c.Callee, key, f.lang) {
			return true
		}
	}
	return false
}

// isSetter reports a method name that begins with set and continues
// with a new word: Set, SetName, set_name, setName; never settle.
func isSetter(name string) bool {
	return startsWithWord(name, "set")
}

// ---- the checks -------------------------------------------------------

func checkTermsDeclared(r rule.Rule, subject rule.Subject, cc contextCode, _ domainCode) ([]Violation, error) {
	var vs []Violation
	for _, t := range cc.ctx.Terms() {
		var what string
		switch {
		case t.Concept == vocab.ConceptEntity:
			what = "entity"
		case t.Concept == vocab.ConceptValueObject && t.Implied:
			what = "identity"
		case t.Concept == vocab.ConceptValueObject:
			what = "value object"
		default:
			continue
		}
		more, err := cc.locationViolations(r, subject, what, t.Name, t.Line, cc.terms[t.Name], "type",
			fmt.Sprintf("declare a type %s, or remove %s from the recorded language", t.Name, t.Name))
		if err != nil {
			return nil, err
		}
		vs = append(vs, more...)
	}
	return vs, nil
}

// checkCodeHeldByOneContext reports, once per other context, the files
// of this context that the other holds too, unless the two record a
// shared_kernel relation; the finding anchors at this context's record.
func checkCodeHeldByOneContext(r rule.Rule, subject rule.Subject, cc contextCode, code domainCode) ([]Violation, error) {
	self := cc.ctx.Name
	shared := map[string][]string{}
	var others []string
	for _, f := range cc.files {
		for _, other := range code.holders[f] {
			if other == self {
				continue
			}
			if _, seen := shared[other]; !seen {
				others = append(others, other)
			}
			shared[other] = append(shared[other], f)
		}
	}
	sort.Strings(others)
	var vs []Violation
	for _, other := range others {
		if rel, ok := code.knowledge.Relation(self, other); ok && rel.Kind == vocab.RelationSharedKernel {
			continue
		}
		files := shared[other]
		file, line := recordAnchor(cc.ctx.Line)
		v, err := newViolation(r, subject, file, line,
			fmt.Sprintf("context %s holds %d file(s) that context %s holds too (%s), and the context map records no shared_kernel between them; a boundary two models straddle is not a boundary", self, len(files), other, exampleFiles(files)),
			fmt.Sprintf("narrow %s and %s with a Zone named for each in rules.arclint.yaml, or record a shared_kernel relation between them under relations in %s", self, other, vocab.UbiquitousLanguageFileName))
		if err != nil {
			return nil, err
		}
		vs = append(vs, v)
	}
	return vs, nil
}

// exampleFiles names up to three files, then how many more.
func exampleFiles(files []string) string {
	const shown = 3
	if len(files) <= shown {
		return strings.Join(files, ", ")
	}
	return fmt.Sprintf("%s, and %d more", strings.Join(files[:shown], ", "), len(files)-shown)
}

func checkContextIsolated(r rule.Rule, subject rule.Subject, cc contextCode, code domainCode) ([]Violation, error) {
	var vs []Violation
	self := cc.ctx.Name
	for _, fi := range cc.imports {
		others := code.contextsOf(fi.imp)
		if len(others) == 0 || containsString(others, self) {
			continue
		}
		for _, other := range others {
			if _, ok := code.knowledge.Relation(self, other); ok {
				continue
			}
			v, err := newViolation(r, subject, fi.path, fi.imp.Line,
				fmt.Sprintf("context %s imports context %s (%s), and the context map records no relation between them", self, other, fi.imp.Path),
				fmt.Sprintf("record the relation between %s and %s under relations in %s, or remove the import", self, other, vocab.UbiquitousLanguageFileName))
			if err != nil {
				return nil, err
			}
			vs = append(vs, v)
		}
	}
	return vs, nil
}

func checkImportsFollowInfluence(r rule.Rule, subject rule.Subject, cc contextCode, code domainCode) ([]Violation, error) {
	var vs []Violation
	self := cc.ctx.Name
	for _, fi := range cc.imports {
		others := code.contextsOf(fi.imp)
		if len(others) == 0 || containsString(others, self) {
			continue
		}
		for _, other := range others {
			rel, ok := code.knowledge.Relation(self, other)
			if !ok || rel.Influences(self, other) {
				continue
			}
			v, err := newViolation(r, subject, fi.path, fi.imp.Line,
				fmt.Sprintf("context %s imports context %s (%s) against the recorded %s relation: %s", self, other, fi.imp.Path, rel.Kind, influenceText(rel)),
				"invert the dependency, or change the recorded relation")
			if err != nil {
				return nil, err
			}
			vs = append(vs, v)
		}
	}
	return vs, nil
}

// influenceText says which way a relation lets imports run.
func influenceText(rel vocab.ContextRelation) string {
	if rel.Kind == vocab.RelationSeparateWays {
		return "neither context imports the other"
	}
	return fmt.Sprintf("%s is upstream of %s, and only the downstream imports the upstream", rel.From, rel.To)
}

func checkModelImportsNothingOutside(r rule.Rule, subject rule.Subject, cc contextCode, code domainCode) ([]Violation, error) {
	var vs []Violation
	for _, fi := range cc.imports {
		if len(fi.targets) == 0 || len(code.contextsOf(fi.imp)) > 0 {
			continue
		}
		zones := sortedZones(fi.targets)
		v, err := newViolation(r, subject, fi.path, fi.imp.Line,
			fmt.Sprintf("context %s imports %s, code of Zone(s) %s that no context holds; dependencies run toward the model and never out of it", cc.ctx.Name, fi.imp.Path, quotedZones(zones)),
			fmt.Sprintf("invert the dependency so that %s depends on the model, or make it the model's code by recording the terms it declares in %s", fi.imp.Path, vocab.UbiquitousLanguageFileName))
		if err != nil {
			return nil, err
		}
		vs = append(vs, v)
	}
	return vs, nil
}

// constructorShapes says what a finding accepts as a constructor.
const constructorShapes = "a function returning it, constructor or __init__, or a factory method on it named create, from, of, parse, new, or build"

func checkValueObjectConstructor(r rule.Rule, subject rule.Subject, cc contextCode, _ domainCode) ([]Violation, error) {
	var vs []Violation
	for _, vo := range cc.ctx.ValueObjects {
		if len(vo.Invariants) == 0 {
			continue
		}
		loc := cc.terms[vo.Name]
		if !loc.located() {
			continue
		}
		decl := loc.decl()
		if len(cc.unitIdx(decl).constructors(decl.decl.Name)) > 0 {
			continue
		}
		v, err := newViolation(r, subject, decl.file.path, decl.decl.StartLine,
			fmt.Sprintf("value object %s records invariant %s and declares no constructor; a value that violates it can be built", vo.Name, vo.Invariants[0].Key),
			fmt.Sprintf("declare a constructor for %s (%s) and enforce its invariants there", decl.decl.Name, constructorShapes))
		if err != nil {
			return nil, err
		}
		vs = append(vs, v)
	}
	return vs, nil
}

func checkValueObjectNoSetters(r rule.Rule, subject rule.Subject, cc contextCode, _ domainCode) ([]Violation, error) {
	var vs []Violation
	for _, t := range cc.ctx.Terms() {
		if t.Concept != vocab.ConceptValueObject {
			continue
		}
		loc := cc.terms[t.Name]
		if !loc.located() {
			continue
		}
		more, err := setterViolations(r, subject, cc, loc.decl(), "value object", t.Name, "nothing changes a value after construction")
		if err != nil {
			return nil, err
		}
		vs = append(vs, more...)
	}
	return vs, nil
}

// setterViolations reports every setter declared on a located type in
// its unit.
func setterViolations(r rule.Rule, subject rule.Subject, cc contextCode, decl locDecl, what, term, why string) ([]Violation, error) {
	var vs []Violation
	for _, m := range cc.unitIdx(decl).methodsOn(decl.decl.Name) {
		if !isSetter(m.decl.Name) {
			continue
		}
		v, err := newViolation(r, subject, m.file.path, m.decl.StartLine,
			fmt.Sprintf("%s %s declares setter %s; %s", what, term, m.decl.Name, why),
			fmt.Sprintf("remove %s, or replace it with an operation named for what it does", m.decl.Name))
		if err != nil {
			return nil, err
		}
		vs = append(vs, v)
	}
	return vs, nil
}

func checkInvariantEnforcedAtMutation(r rule.Rule, subject rule.Subject, cc contextCode, _ domainCode) ([]Violation, error) {
	var vs []Violation
	for _, agg := range cc.ctx.Aggregates {
		ac := cc.aggregates[agg.Name]
		if len(agg.Invariants) == 0 || !ac.located() {
			continue
		}
		root := ac.root()
		ctors := ac.idx.constructors(root.decl.Name)
		cmds := ac.idx.commands(agg, root.decl.Name)
		for _, inv := range agg.Invariants {
			key := ensureKey(inv.Key)
			if _, declared, err := ac.idx.methodNamed(root.decl.Name, key); err != nil {
				return nil, err
			} else if !declared {
				continue
			}
			if len(ctors) == 0 {
				v, err := newViolation(r, subject, root.file.path, root.decl.StartLine,
					fmt.Sprintf("aggregate %s declares no constructor (%s), so invariant %s is not enforced when a %s is built", agg.Name, constructorShapes, inv.Key, agg.Name),
					fmt.Sprintf("declare a constructor for %s that calls %s", root.decl.Name, methodSpellings(ac.idx, key)))
				if err != nil {
					return nil, err
				}
				vs = append(vs, v)
			}
			for _, op := range append(append([]locDecl(nil), ctors...), cmds...) {
				if callsKey(op.file, op.decl.Name, key) {
					continue
				}
				name, err := methodNameFor(key, op.file.lang)
				if err != nil {
					return nil, err
				}
				kind := "command"
				if isConstructor(op.decl, root.decl.Name) {
					kind = "constructor"
				}
				v, err := newViolation(r, subject, op.file.path, op.decl.StartLine,
					fmt.Sprintf("aggregate %s: %s %s does not call %s, so invariant %s is not enforced when it completes", agg.Name, kind, op.decl.Name, name, inv.Key),
					fmt.Sprintf("call %s from %s and fail the operation when it fails", name, op.decl.Name))
				if err != nil {
					return nil, err
				}
				vs = append(vs, v)
			}
		}
	}
	return vs, nil
}

func checkAssertionChecked(r rule.Rule, subject rule.Subject, cc contextCode, _ domainCode) ([]Violation, error) {
	var vs []Violation
	for _, agg := range cc.ctx.Aggregates {
		ac := cc.aggregates[agg.Name]
		if !ac.located() {
			continue
		}
		root := ac.root()
		for _, as := range agg.Assertions {
			key := assertKey(as.Key)
			op, hasOp, err := ac.idx.methodNamed(root.decl.Name, as.On)
			if err != nil {
				return nil, err
			}
			check, hasCheck, err := ac.idx.methodNamed(root.decl.Name, key)
			if err != nil {
				return nil, err
			}
			if !hasOp {
				v, err := newViolation(r, subject, root.file.path, root.decl.StartLine,
					fmt.Sprintf("aggregate %s: assertion %s constrains operation %s, which the root does not declare", agg.Name, as.Key, as.On),
					fmt.Sprintf("declare %s on %s and call %s from it", methodSpellings(ac.idx, as.On), root.decl.Name, methodSpellings(ac.idx, key)))
				if err != nil {
					return nil, err
				}
				vs = append(vs, v)
			}
			if !hasCheck {
				v, err := newViolation(r, subject, root.file.path, root.decl.StartLine,
					fmt.Sprintf("aggregate %s: assertion %s names no checking method on the root; expected %s", agg.Name, as.Key, methodSpellings(ac.idx, key)),
					fmt.Sprintf("declare %s on %s and call it from %s", methodSpellings(ac.idx, key), root.decl.Name, as.On))
				if err != nil {
					return nil, err
				}
				vs = append(vs, v)
			}
			if !hasOp || !hasCheck {
				continue
			}
			if callsKey(op.file, op.decl.Name, key) {
				continue
			}
			v, err := newViolation(r, subject, op.file.path, op.decl.StartLine,
				fmt.Sprintf("aggregate %s: operation %s does not call %s, so assertion %s is not checked when it completes", agg.Name, op.decl.Name, check.decl.Name, as.Key),
				fmt.Sprintf("call %s from %s and fail the operation when it fails", check.decl.Name, op.decl.Name))
			if err != nil {
				return nil, err
			}
			vs = append(vs, v)
		}
	}
	return vs, nil
}

func checkSpecificationSatisfaction(r rule.Rule, subject rule.Subject, cc contextCode, _ domainCode) ([]Violation, error) {
	var vs []Violation
	for _, s := range cc.ctx.Specifications {
		loc := cc.terms[s.Name]
		more, err := cc.locationViolations(r, subject, "specification", s.Name, s.Line, loc, "type",
			fmt.Sprintf("declare a type %s carrying SatisfiedBy, satisfiedBy, or satisfied_by", s.Name))
		if err != nil {
			return nil, err
		}
		vs = append(vs, more...)
		if !loc.located() {
			continue
		}
		decl := loc.decl()
		if hasSatisfaction(cc.unitIdx(decl), decl.decl.Name) {
			continue
		}
		v, err := newViolation(r, subject, decl.file.path, decl.decl.StartLine,
			fmt.Sprintf("specification %s declares no satisfaction method", s.Name),
			fmt.Sprintf("declare SatisfiedBy, satisfiedBy, or satisfied_by on %s", decl.decl.Name))
		if err != nil {
			return nil, err
		}
		vs = append(vs, v)
	}
	return vs, nil
}

func hasSatisfaction(idx contractIndex, typeName string) bool {
	for _, m := range idx.methodsOn(typeName) {
		if isSatisfaction(m.decl.Name) {
			return true
		}
	}
	return false
}

// isSatisfaction reports whether a method name is a specification's
// satisfaction method in any published language.
func isSatisfaction(name string) bool {
	switch name {
	case "SatisfiedBy", "satisfiedBy", "satisfied_by":
		return true
	}
	return false
}

func checkAggregateProtectsInvariant(r rule.Rule, subject rule.Subject, cc contextCode, _ domainCode) ([]Violation, error) {
	var vs []Violation
	for _, agg := range cc.ctx.Aggregates {
		if len(agg.Invariants) > 0 {
			continue
		}
		file, line := recordAnchor(agg.Line)
		v, err := newViolation(r, subject, file, line,
			fmt.Sprintf("aggregate %s of context %s records no invariant; a boundary drawn around nothing that must stay consistent is not yet justified", agg.Name, cc.ctx.Name),
			fmt.Sprintf("record what %s keeps consistent under invariants, or record it as a value object", agg.Name))
		if err != nil {
			return nil, err
		}
		vs = append(vs, v)
	}
	return vs, nil
}

func checkAggregateRootDeclared(r rule.Rule, subject rule.Subject, cc contextCode, _ domainCode) ([]Violation, error) {
	var vs []Violation
	for _, agg := range cc.ctx.Aggregates {
		ac := cc.aggregates[agg.Name]
		if ac.located() {
			continue
		}
		file, line := recordAnchor(agg.Line)
		if len(ac.candidates) > 1 {
			v, err := newViolation(r, subject, file, line,
				fmt.Sprintf("aggregate %s of context %s has %d candidate roots in %s (%s), and nothing tells which is the model's", agg.Name, cc.ctx.Name, len(ac.candidates), cc.searched(), candidateList(ac.candidates)),
				cc.narrowRemedy())
			if err != nil {
				return nil, err
			}
			vs = append(vs, v)
			continue
		}
		v, err := newViolation(r, subject, file, line,
			fmt.Sprintf("aggregate %s of context %s has no root: no type %s is declared in %s", agg.Name, cc.ctx.Name, agg.Name, cc.searched()),
			fmt.Sprintf("declare the root type %s (a struct in Go, a class in TypeScript or Python)", agg.Name))
		if err != nil {
			return nil, err
		}
		vs = append(vs, v)
	}
	return vs, nil
}

func checkAggregateInvariantsOnRoot(r rule.Rule, subject rule.Subject, cc contextCode, _ domainCode) ([]Violation, error) {
	var vs []Violation
	for _, agg := range cc.ctx.Aggregates {
		ac := cc.aggregates[agg.Name]
		if !ac.located() {
			continue
		}
		root := ac.root()
		for _, inv := range agg.Invariants {
			key := ensureKey(inv.Key)
			_, declared, err := ac.idx.methodNamed(root.decl.Name, key)
			if err != nil {
				return nil, err
			}
			if declared {
				continue
			}
			v, err := newViolation(r, subject, root.file.path, root.decl.StartLine,
				fmt.Sprintf("aggregate %s: invariant %s is not enforced by a method of the root; expected %s", agg.Name, inv.Key, methodSpellings(ac.idx, key)),
				fmt.Sprintf("declare %s on %s, returning the failure, and call it from every constructor and command", methodSpellings(ac.idx, key), root.decl.Name))
			if err != nil {
				return nil, err
			}
			vs = append(vs, v)
		}
	}
	return vs, nil
}

func checkAggregateNoSetters(r rule.Rule, subject rule.Subject, cc contextCode, _ domainCode) ([]Violation, error) {
	var vs []Violation
	for _, agg := range cc.ctx.Aggregates {
		ac := cc.aggregates[agg.Name]
		if !ac.located() {
			continue
		}
		more, err := setterViolations(r, subject, cc, ac.root(), "aggregate root", agg.Name, "a change of state is a command named for what it does")
		if err != nil {
			return nil, err
		}
		vs = append(vs, more...)
	}
	return vs, nil
}

// checkReferencesByIdentity reports every constructor and command of a
// root that takes or returns the root of another aggregate, anywhere
// in the recorded domain. In the aggregate's own unit the other root
// is named bare; from another Go package it is named through the
// package the file imports; from another TypeScript or Python module
// it is named bare in a file importing that module.
func checkReferencesByIdentity(r rule.Rule, subject rule.Subject, cc contextCode, code domainCode) ([]Violation, error) {
	var vs []Violation
	for _, agg := range cc.ctx.Aggregates {
		ac := cc.aggregates[agg.Name]
		if !ac.located() {
			continue
		}
		root := ac.root()
		ops := append(ac.idx.constructors(root.decl.Name), ac.idx.commands(agg, root.decl.Name)...)
		for _, op := range ops {
			for _, other := range code.roots {
				if other.root.file.path == root.file.path && other.root.decl.StartLine == root.decl.StartLine {
					continue
				}
				text, result, found := cc.rootReference(op, root, other)
				if !found {
					continue
				}
				kind, verb := "command", "takes"
				if isConstructor(op.decl, root.decl.Name) {
					kind = "constructor"
				}
				if result {
					verb = "returns"
				}
				v, err := newViolation(r, subject, op.file.path, op.decl.StartLine,
					fmt.Sprintf("aggregate %s: %s %s %s %s, the root of aggregate %s; what one aggregate needs of another it receives as an identity or a value", agg.Name, kind, op.decl.Name, verb, text, other.agg.Name),
					fmt.Sprintf("pass %s's identity (%s) or a value into %s instead of its root", other.agg.Name, other.agg.Identity, agg.Name))
				if err != nil {
					return nil, err
				}
				vs = append(vs, v)
			}
		}
	}
	return vs, nil
}

// rootReference finds the parameter or result of an operation that
// names another aggregate's root, returning its type text and whether
// it is a result.
func (cc contextCode) rootReference(op, root locDecl, other aggregateRoot) (string, bool, bool) {
	qualifier := ""
	if path.Dir(op.file.path) != other.unit {
		if !cc.importsUnit(op.file.path, other.unit) {
			return "", false, false
		}
		if op.file.lang == rule.LanguageGo {
			qualifier = other.root.file.pkg
		}
	}
	if qualifier == "" && other.root.decl.Name == root.decl.Name {
		// Two roots of one name: a bare token in this file is this
		// file's own root, and the other cannot be told apart from it
		// without the import's binding.
		return "", false, false
	}
	for _, p := range op.decl.Params {
		if mentionsType(p.Type, qualifier, other.root.decl.Name) {
			return p.Type, false, true
		}
	}
	for _, res := range op.decl.Results {
		if mentionsType(res, qualifier, other.root.decl.Name) {
			return res, true, true
		}
	}
	return "", false, false
}

func checkEventDeclared(r rule.Rule, subject rule.Subject, cc contextCode, _ domainCode) ([]Violation, error) {
	var vs []Violation
	for _, e := range cc.ctx.Events {
		more, err := cc.locationViolations(r, subject, "event", e.Name, e.Line, cc.terms[e.Name], "type",
			fmt.Sprintf("declare a type %s, or remove %s from the recorded language", e.Name, e.Name))
		if err != nil {
			return nil, err
		}
		vs = append(vs, more...)
	}
	return vs, nil
}

func checkEventNoSetters(r rule.Rule, subject rule.Subject, cc contextCode, _ domainCode) ([]Violation, error) {
	var vs []Violation
	for _, e := range cc.ctx.Events {
		loc := cc.terms[e.Name]
		if !loc.located() {
			continue
		}
		more, err := setterViolations(r, subject, cc, loc.decl(), "event", e.Name, "a record of the past is not edited")
		if err != nil {
			return nil, err
		}
		vs = append(vs, more...)
	}
	return vs, nil
}

func checkServiceDeclared(r rule.Rule, subject rule.Subject, cc contextCode, _ domainCode) ([]Violation, error) {
	var vs []Violation
	for _, s := range cc.ctx.Services {
		more, err := cc.locationViolations(r, subject, "service", s.Name, s.Line, cc.terms[s.Name], "type",
			fmt.Sprintf("declare a type %s, or remove %s from the recorded language", s.Name, s.Name))
		if err != nil {
			return nil, err
		}
		vs = append(vs, more...)
	}
	return vs, nil
}

func checkRepositoryDeclared(r rule.Rule, subject rule.Subject, cc contextCode, _ domainCode) ([]Violation, error) {
	var vs []Violation
	for _, agg := range cc.ctx.Aggregates {
		if agg.Repository == "" {
			continue
		}
		loc := cc.terms[agg.Repository]
		more, err := cc.locationViolations(r, subject, "repository", agg.Repository, agg.Line, loc, "type",
			fmt.Sprintf("declare %s as an interface (a class in a language without interfaces)", agg.Repository))
		if err != nil {
			return nil, err
		}
		vs = append(vs, more...)
		if !loc.located() {
			continue
		}
		decl := loc.decl()
		if repositoryKind(decl.decl.Kind, decl.file.lang) {
			continue
		}
		v, err := newViolation(r, subject, decl.file.path, decl.decl.StartLine,
			fmt.Sprintf("repository %s of aggregate %s is declared as a %s; a repository is an interface the model owns and the outside implements", agg.Repository, agg.Name, decl.decl.Kind),
			fmt.Sprintf("declare %s as an interface (a class in a language without interfaces)", decl.decl.Name))
		if err != nil {
			return nil, err
		}
		vs = append(vs, v)
	}
	return vs, nil
}

// repositoryKind reports whether a declaration kind can be a repository
// in a language: an interface, or a class where the language declares
// abstractions as classes.
func repositoryKind(kind string, lang rule.Language) bool {
	switch lang {
	case rule.LanguageGo:
		return kind == declKindInterface
	case rule.LanguageTypeScript:
		return kind == declKindInterface || kind == declKindClass
	case rule.LanguagePython:
		return kind == declKindClass
	}
	return kind == declKindInterface || kind == declKindClass
}

func checkFactoryDeclared(r rule.Rule, subject rule.Subject, cc contextCode, _ domainCode) ([]Violation, error) {
	var vs []Violation
	for _, agg := range cc.ctx.Aggregates {
		if agg.Factory == "" {
			continue
		}
		more, err := cc.locationViolations(r, subject, "factory", agg.Factory, agg.Line, cc.terms[agg.Factory], "type or function",
			fmt.Sprintf("declare %s as a type or a function that returns %s", agg.Factory, agg.Name))
		if err != nil {
			return nil, err
		}
		vs = append(vs, more...)
	}
	return vs, nil
}

func containsString(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
