package vocab

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

//go:generate go run ../../../tools/genmetamodel -in metamodel.arclint.yaml -out metamodel_gen.go -pkg vocab

// MetaModel is the language arclint speaks about a project's domain:
// the building blocks of Domain-Driven Design, each defined with its
// sources, the shape a project records an instance of it in, the
// invariants arclint applies to every recorded instance, and the
// guidance that is advice rather than a checkable proposition. The one
// instance is DDD, generated from metamodel.arclint.yaml; every
// artifact that explains the language is derived from it.
type MetaModel struct {
	Name        string
	Title       string
	Description string
	Works       []Work
	Facts       []FactDefinition
	Milestones  []Milestone
	Phases      []Phase
	Blocks      []BuildingBlock
}

// Work is one entry of the meta-model's bibliography, the target of
// every Citation. Verification says how the cited pages were checked
// while the meta-model was written.
type Work struct {
	Key          string
	Title        string
	Authors      []string
	Publisher    string
	Year         int
	Date         string
	ISBN         string
	URL          string
	License      string
	Verification Verification
}

// Verification is how a Work's cited pages were checked.
type Verification string

const (
	// VerifiedText means the work's text was extracted and the cited
	// passages read.
	VerifiedText Verification = "text"
	// VerifiedFetched means the page was retrieved and the quoted
	// sentences read.
	VerifiedFetched Verification = "fetched"
	// VerifiedCarried means the chapter and page numbers were carried
	// from the draft the meta-model was made from and are unchecked
	// against the print edition.
	VerifiedCarried Verification = "carried"
)

// Citation points a definition, an invariant, a guidance entry, or a
// relation kind at the Work that states it. Work is required; Chapter,
// Page, and Section narrow the reference. Page is text because the
// Reference's front matter is numbered in roman numerals.
type Citation struct {
	Work    string
	Chapter int
	Page    string
	Section string
}

// Reference is a Citation resolved against the Work it names, the form
// a reader is shown.
type Reference struct {
	Work    Work
	Chapter int
	Page    string
	Section string
}

// String spells the reference for a reader: the authors, the title, the
// year or date, the narrowing parts, and the URL when the work has one.
func (r Reference) String() string {
	var b strings.Builder
	if len(r.Work.Authors) > 0 {
		b.WriteString(strings.Join(r.Work.Authors, " and "))
		b.WriteString(", ")
	}
	b.WriteString(r.Work.Title)
	switch {
	case r.Work.Date != "":
		b.WriteString(" (" + r.Work.Date + ")")
	case r.Work.Year != 0:
		b.WriteString(" (" + strconv.Itoa(r.Work.Year) + ")")
	}
	if r.Chapter != 0 {
		b.WriteString(", ch. " + strconv.Itoa(r.Chapter))
	}
	if r.Page != "" {
		b.WriteString(", p. " + r.Page)
	}
	if r.Section != "" {
		b.WriteString(", " + r.Section)
	}
	if r.Work.URL != "" {
		b.WriteString(" <" + r.Work.URL + ">")
	}
	return b.String()
}

// Resolve looks up the Work a Citation names; false when the meta-model
// lists no such work.
func (m MetaModel) Resolve(c Citation) (Reference, bool) {
	w, ok := m.Work(c.Work)
	if !ok {
		return Reference{}, false
	}
	return Reference{Work: w, Chapter: c.Chapter, Page: c.Page, Section: c.Section}, true
}

// References resolves citations in order, leaving out any that names an
// unlisted work; a validated meta-model has none.
func (m MetaModel) References(cs []Citation) []Reference {
	refs := make([]Reference, 0, len(cs))
	for _, c := range cs {
		if r, ok := m.Resolve(c); ok {
			refs = append(refs, r)
		}
	}
	return refs
}

// FactDefinition is one fact arclint observes in source, named as in
// the Rule context's enforcement table, with the languages that emit it.
type FactDefinition struct {
	Name       string
	Definition string
	Languages  Languages
}

// Milestone is a fact arclint does not observe yet. The invariants it
// unlocks are sourced and binary; only the observation is missing.
type Milestone struct {
	Key     string
	Needs   string
	Unlocks []string
}

// Phase groups building blocks: strategic design, tactical design, and
// arclint's own structural vocabulary.
type Phase struct {
	Name       string
	Title      string
	Definition Meaning
}

// Meaning is a definition with the sources that state it: the
// `definition` of a phase or a building block. A structural-phase
// block has no sources because arclint is its source.
type Meaning struct {
	Text    string
	Sources []Citation
}

// BuildingBlock is one term of the canonical language.
type BuildingBlock struct {
	Term       string
	Title      string
	Phase      string
	Definition Meaning
	Records    Recording
	Invariants []BlockInvariant
	Guidance   []Guidance
}

// Recording is how a project records an instance of a building block in
// its domain file: the section the instance is written under and the
// properties the instance carries. Kinds lists the published values of
// a block's kind property.
type Recording struct {
	Section    Section
	Note       string
	Properties []Property
	Kinds      []KindDefinition
}

// Section is the place in the domain file where instances of a building
// block are recorded.
type Section string

const (
	// SectionTop is the domain file itself.
	SectionTop Section = "top"
	// SectionNone marks a block that has no entry of its own.
	SectionNone Section = "none"
	// SectionContexts is the contexts map, keyed by context name.
	SectionContexts Section = "contexts"
	// SectionRelations is the relations list.
	SectionRelations Section = "relations"
	// SectionEntities is an aggregate's entities map.
	SectionEntities Section = "entities"
	// SectionValueObjects is a context's value_objects map.
	SectionValueObjects Section = "value_objects"
	// SectionInvariants is an owner's invariants map.
	SectionInvariants Section = "invariants"
	// SectionAssertions is an aggregate's assertions map.
	SectionAssertions Section = "assertions"
	// SectionSpecifications is a context's specifications map.
	SectionSpecifications Section = "specifications"
	// SectionAggregates is a context's aggregates map.
	SectionAggregates Section = "aggregates"
	// SectionEvents is a context's events map.
	SectionEvents Section = "events"
	// SectionServices is a context's services map.
	SectionServices Section = "services"
	// SectionQuestions is a context's questions map.
	SectionQuestions Section = "questions"
)

// Sections lists every published Section in the order the domain file
// documents them.
func Sections() []Section {
	return []Section{
		SectionTop, SectionNone, SectionContexts, SectionRelations,
		SectionEntities, SectionValueObjects, SectionInvariants,
		SectionAssertions, SectionSpecifications, SectionAggregates,
		SectionEvents, SectionServices, SectionQuestions,
	}
}

// Property is one recorded property of a building block instance.
type Property struct {
	Name       string
	Required   bool
	Definition string
}

// KindDefinition is one published value of a block's kind property,
// defined with its sources.
type KindDefinition struct {
	Name       string
	Definition string
	Sources    []Citation
}

// BlockInvariant is one structural rule arclint applies to every
// recorded instance of a building block, cited to the source that
// states it and classified by how it is enforced. Its ID is the block's
// term, a slash, and the rule's kebab-case name.
type BlockInvariant struct {
	ID          string
	Statement   string
	Sources     []Citation
	Enforcement Enforcement
}

// Enforcement classifies how a BlockInvariant is evaluated. By names
// the evaluator; Facts are the observed facts the invariant reads;
// Needs names the Milestone an unobserved fact belongs to; Languages
// says which supported languages emit the facts; Severity is error or
// warning, and a warning never rejects an instance or fails a check.
type Enforcement struct {
	By        Evaluator
	Facts     []string
	Languages Languages
	Needs     string
	Severity  string
}

// The two severities a BlockInvariant carries; they are the Rule
// severities of the same names, spelled here because the rule package
// depends on this one.
const (
	severityError   = "error"
	severityWarning = "warning"
)

// Evaluator names what evaluates a BlockInvariant.
type Evaluator string

const (
	// EvaluatorLoader evaluates the invariant against the domain file
	// alone, while it is loaded; a recording that breaks it is refused.
	EvaluatorLoader Evaluator = "loader"
	// EvaluatorDomain is the domain evaluator of `arclint check`: one
	// built-in Rule per invariant, applied to every recorded instance
	// the moment the domain file records one.
	EvaluatorDomain Evaluator = "domain"
	// EvaluatorPlanned means the facts suffice and no evaluator exists
	// yet.
	EvaluatorPlanned Evaluator = "planned"
)

// Evaluators lists every published Evaluator in the order the
// meta-model documents them.
func Evaluators() []Evaluator {
	return []Evaluator{EvaluatorLoader, EvaluatorDomain, EvaluatorPlanned}
}

// EnforcementLevel is where a BlockInvariant is evaluated. It is never
// recorded; Level derives it from the invariant's Enforcement.
type EnforcementLevel string

const (
	// LevelRecording means the loader evaluates the invariant against
	// the domain file alone.
	LevelRecording EnforcementLevel = "recording"
	// LevelCheck means `arclint check` evaluates the invariant against
	// source.
	LevelCheck EnforcementLevel = "check"
	// LevelMilestone means the fact the invariant needs is not observed
	// yet.
	LevelMilestone EnforcementLevel = "milestone"
)

// Languages says which supported languages an invariant or a fact
// covers: every one, or the named ones. Neither is set for a milestone
// invariant no language emits the facts for yet.
type Languages struct {
	All   bool
	Names []string
}

// Guidance is design advice about a building block, cited, never
// enforced. Fundamental marks the pattern's own directive: an instance
// that ignores it misuses the block even though arclint cannot observe
// the misuse.
type Guidance struct {
	Text        string
	Fundamental bool
	Sources     []Citation
}

// Level derives where the invariant is evaluated: a milestone when it
// needs an unobserved fact, recording when the loader evaluates it, and
// check otherwise.
func (i BlockInvariant) Level() EnforcementLevel {
	switch {
	case i.Enforcement.Needs != "":
		return LevelMilestone
	case i.Enforcement.By == EvaluatorLoader:
		return LevelRecording
	default:
		return LevelCheck
	}
}

// Term is the building block the invariant belongs to, the part of its
// ID before the slash.
func (i BlockInvariant) Term() string {
	term, _, _ := strings.Cut(i.ID, "/")
	return term
}

// Block returns the building block recorded under term.
func (m MetaModel) Block(term string) (BuildingBlock, bool) {
	for _, b := range m.Blocks {
		if b.Term == term {
			return b, true
		}
	}
	return BuildingBlock{}, false
}

// Invariant returns the block invariant with the given ID.
func (m MetaModel) Invariant(id string) (BlockInvariant, bool) {
	for _, b := range m.Blocks {
		for _, inv := range b.Invariants {
			if inv.ID == id {
				return inv, true
			}
		}
	}
	return BlockInvariant{}, false
}

// Invariants lists every block invariant in file order.
func (m MetaModel) Invariants() []BlockInvariant {
	var all []BlockInvariant
	for _, b := range m.Blocks {
		all = append(all, b.Invariants...)
	}
	return all
}

// Work returns the bibliography entry with the given key.
func (m MetaModel) Work(key string) (Work, bool) {
	for _, w := range m.Works {
		if w.Key == key {
			return w, true
		}
	}
	return Work{}, false
}

// Milestone returns the milestone with the given key.
func (m MetaModel) Milestone(key string) (Milestone, bool) {
	for _, ms := range m.Milestones {
		if ms.Key == key {
			return ms, true
		}
	}
	return Milestone{}, false
}

// Fact returns the fact definition with the given name.
func (m MetaModel) Fact(name string) (FactDefinition, bool) {
	for _, f := range m.Facts {
		if f.Name == name {
			return f, true
		}
	}
	return FactDefinition{}, false
}

// Phase returns the phase with the given name.
func (m MetaModel) Phase(name string) (Phase, bool) {
	for _, p := range m.Phases {
		if p.Name == name {
			return p, true
		}
	}
	return Phase{}, false
}

// DDD is the Domain-Driven Design meta-model this binary speaks,
// generated from metamodel.arclint.yaml. Every call returns a fresh
// value, so a caller may reorder or trim what it receives without
// touching what the next caller sees.
func DDD() MetaModel {
	return ddd()
}

// The structural phase is arclint's own vocabulary: its blocks cite
// nothing because arclint is their source, and every other block
// cites at least one work.
const structuralPhase = "structural"

var (
	termPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	keyPattern  = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
	rulePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
)

// Validate checks the meta-model as one whole and returns every failure
// joined: a term or an id recorded twice, a citation of an unlisted
// work, an invariant whose id is not prefixed by its block's term, a
// milestone and an invariant that do not name each other, an evaluator
// or a severity outside the published values, and an enforcement whose
// facts contradict its evaluator. The generated table is validated by a
// test, so a meta-model that fails here never ships.
func (m MetaModel) Validate() error {
	v := &metaModelValidation{model: m}
	v.header()
	v.works()
	v.facts()
	v.milestones()
	v.phases()
	v.blocks()
	v.milestonesUnlockRecordedInvariants()
	return errors.Join(v.failures...)
}

type metaModelValidation struct {
	model      MetaModel
	failures   []error
	invariants map[string]BlockInvariant
}

func (v *metaModelValidation) failf(format string, args ...any) {
	v.failures = append(v.failures, fmt.Errorf(format, args...))
}

func (v *metaModelValidation) header() {
	if strings.TrimSpace(v.model.Name) == "" {
		v.failf("meta-model: name must be non-empty")
	}
	if strings.TrimSpace(v.model.Title) == "" {
		v.failf("meta-model: title must be non-empty")
	}
	if strings.TrimSpace(v.model.Description) == "" {
		v.failf("meta-model: description must be non-empty")
	}
}

func (v *metaModelValidation) works() {
	seen := map[string]bool{}
	for _, w := range v.model.Works {
		switch {
		case !keyPattern.MatchString(w.Key):
			v.failf("works: key %q is not lowercase kebab or snake case", w.Key)
		case seen[w.Key]:
			v.failf("works: key %q recorded twice", w.Key)
		}
		seen[w.Key] = true
		if strings.TrimSpace(w.Title) == "" {
			v.failf("works %q: title must be non-empty", w.Key)
		}
		if len(w.Authors) == 0 {
			v.failf("works %q: at least one author is required", w.Key)
		}
		if w.Year == 0 && w.Date == "" {
			v.failf("works %q: a year or a date is required", w.Key)
		}
		switch w.Verification {
		case VerifiedText, VerifiedFetched, VerifiedCarried:
		default:
			v.failf("works %q: verification %q is not text, fetched, or carried", w.Key, w.Verification)
		}
	}
}

func (v *metaModelValidation) facts() {
	seen := map[string]bool{}
	for _, f := range v.model.Facts {
		switch {
		case !keyPattern.MatchString(f.Name):
			v.failf("facts: name %q is not lowercase snake case", f.Name)
		case seen[f.Name]:
			v.failf("facts: name %q recorded twice", f.Name)
		}
		seen[f.Name] = true
		if strings.TrimSpace(f.Definition) == "" {
			v.failf("facts %q: definition must be non-empty", f.Name)
		}
		if !f.Languages.All && len(f.Languages.Names) == 0 {
			v.failf("facts %q: languages must be all or name at least one language", f.Name)
		}
	}
}

func (v *metaModelValidation) milestones() {
	seen := map[string]bool{}
	for _, ms := range v.model.Milestones {
		switch {
		case !keyPattern.MatchString(ms.Key):
			v.failf("milestones: key %q is not lowercase kebab case", ms.Key)
		case seen[ms.Key]:
			v.failf("milestones: key %q recorded twice", ms.Key)
		}
		seen[ms.Key] = true
		if strings.TrimSpace(ms.Needs) == "" {
			v.failf("milestones %q: needs must say what observation is missing", ms.Key)
		}
		if len(ms.Unlocks) == 0 {
			v.failf("milestones %q: unlocks must name at least one invariant", ms.Key)
		}
	}
}

func (v *metaModelValidation) phases() {
	seen := map[string]bool{}
	for _, p := range v.model.Phases {
		switch {
		case !termPattern.MatchString(p.Name):
			v.failf("phases: name %q is not lowercase snake case", p.Name)
		case seen[p.Name]:
			v.failf("phases: name %q recorded twice", p.Name)
		}
		seen[p.Name] = true
		if strings.TrimSpace(p.Title) == "" {
			v.failf("phases %q: title must be non-empty", p.Name)
		}
		v.meaning("phases "+p.Name, p.Definition, p.Name != structuralPhase)
	}
}

// meaning checks a definition's text and citations; cited says whether
// at least one source is required.
func (v *metaModelValidation) meaning(subject string, d Meaning, cited bool) {
	if strings.TrimSpace(d.Text) == "" {
		v.failf("%s: definition text must be non-empty", subject)
	}
	if cited && len(d.Sources) == 0 {
		v.failf("%s: definition must cite at least one work", subject)
	}
	if !cited && len(d.Sources) > 0 {
		v.failf("%s: a structural term cites nothing, because arclint is its source", subject)
	}
	v.citations(subject, d.Sources)
}

func (v *metaModelValidation) citations(subject string, cs []Citation) {
	for _, c := range cs {
		if _, ok := v.model.Work(c.Work); !ok {
			v.failf("%s: cites work %q, which the bibliography does not list", subject, c.Work)
		}
		if c.Chapter < 0 {
			v.failf("%s: cites a negative chapter of %q", subject, c.Work)
		}
	}
}

func (v *metaModelValidation) blocks() {
	v.invariants = map[string]BlockInvariant{}
	seenTerms := map[string]bool{}
	for _, b := range v.model.Blocks {
		subject := "building_blocks " + b.Term
		switch {
		case !termPattern.MatchString(b.Term):
			v.failf("building_blocks: term %q is not lowercase snake case", b.Term)
		case seenTerms[b.Term]:
			v.failf("building_blocks: term %q recorded twice", b.Term)
		}
		seenTerms[b.Term] = true
		if strings.TrimSpace(b.Title) == "" {
			v.failf("%s: title must be non-empty", subject)
		}
		if _, ok := v.model.Phase(b.Phase); !ok {
			v.failf("%s: phase %q is not a recorded phase", subject, b.Phase)
		}
		v.meaning(subject, b.Definition, b.Phase != structuralPhase)
		v.recording(subject, b.Records)
		for _, inv := range b.Invariants {
			v.invariant(b, inv)
		}
		for i, g := range b.Guidance {
			gsubject := fmt.Sprintf("%s guidance %d", subject, i+1)
			if strings.TrimSpace(g.Text) == "" {
				v.failf("%s: text must be non-empty", gsubject)
			}
			if len(g.Sources) == 0 {
				v.failf("%s: guidance must cite at least one work", gsubject)
			}
			v.citations(gsubject, g.Sources)
		}
	}
}

func (v *metaModelValidation) recording(subject string, r Recording) {
	if !slices.Contains(Sections(), r.Section) {
		v.failf("%s: records.section %q is not a published section, top, or none", subject, r.Section)
	}
	seen := map[string]bool{}
	for _, p := range r.Properties {
		switch {
		case !termPattern.MatchString(p.Name):
			v.failf("%s: property %q is not lowercase snake case", subject, p.Name)
		case seen[p.Name]:
			v.failf("%s: property %q recorded twice", subject, p.Name)
		}
		seen[p.Name] = true
		if strings.TrimSpace(p.Definition) == "" {
			v.failf("%s: property %q must carry a definition", subject, p.Name)
		}
	}
	seenKinds := map[string]bool{}
	for _, k := range r.Kinds {
		switch {
		case !termPattern.MatchString(k.Name):
			v.failf("%s: kind %q is not lowercase snake case", subject, k.Name)
		case seenKinds[k.Name]:
			v.failf("%s: kind %q recorded twice", subject, k.Name)
		}
		seenKinds[k.Name] = true
		if strings.TrimSpace(k.Definition) == "" {
			v.failf("%s: kind %q must carry a definition", subject, k.Name)
		}
		if len(k.Sources) == 0 {
			v.failf("%s: kind %q must cite at least one work", subject, k.Name)
		}
		v.citations(subject+" kind "+k.Name, k.Sources)
	}
}

func (v *metaModelValidation) invariant(b BuildingBlock, inv BlockInvariant) {
	subject := "invariant " + inv.ID
	term, name, found := strings.Cut(inv.ID, "/")
	switch {
	case !found || term != b.Term:
		v.failf("%s: id must be prefixed by its block's term %q and a slash", subject, b.Term)
	case !rulePattern.MatchString(name):
		v.failf("%s: name %q after the slash is not lowercase kebab case", subject, name)
	}
	if _, dup := v.invariants[inv.ID]; dup {
		v.failf("%s: id recorded twice", subject)
	}
	v.invariants[inv.ID] = inv
	if strings.TrimSpace(inv.Statement) == "" {
		v.failf("%s: statement must be non-empty", subject)
	}
	if len(inv.Sources) == 0 {
		v.failf("%s: an invariant must cite at least one work", subject)
	}
	v.citations(subject, inv.Sources)
	v.enforcement(subject, inv)
}

func (v *metaModelValidation) enforcement(subject string, inv BlockInvariant) {
	e := inv.Enforcement
	switch e.Severity {
	case severityError, severityWarning:
	default:
		v.failf("%s: severity %q is not error or warning", subject, e.Severity)
	}
	for _, f := range e.Facts {
		if _, ok := v.model.Fact(f); !ok {
			v.failf("%s: reads fact %q, which arclint does not observe", subject, f)
		}
	}
	if e.Languages.All && len(e.Languages.Names) > 0 {
		v.failf("%s: languages is all and also names languages", subject)
	}
	if inv.Level() == LevelMilestone {
		v.milestoneEnforcement(subject, inv)
		return
	}
	if e.By == "" {
		v.failf("%s: an invariant that needs no milestone names its evaluator", subject)
	}
	if !e.Languages.All && len(e.Languages.Names) == 0 {
		v.failf("%s: languages must be all or name at least one language", subject)
	}
	switch e.By {
	case "":
	case EvaluatorLoader:
		if len(e.Facts) > 0 {
			v.failf("%s: the loader reads the domain file alone; facts %v contradict it", subject, e.Facts)
		}
	case EvaluatorDomain, EvaluatorPlanned:
	default:
		v.failf("%s: evaluator %q is not published", subject, e.By)
	}
}

func (v *metaModelValidation) milestoneEnforcement(subject string, inv BlockInvariant) {
	e := inv.Enforcement
	if e.By != "" {
		v.failf("%s: a milestone invariant names no evaluator until the milestone lands; by %q", subject, e.By)
	}
	ms, ok := v.model.Milestone(e.Needs)
	if !ok {
		v.failf("%s: needs milestone %q, which is not recorded", subject, e.Needs)
		return
	}
	for _, id := range ms.Unlocks {
		if id == inv.ID {
			return
		}
	}
	v.failf("%s: needs milestone %q, but the milestone does not list it under unlocks", subject, e.Needs)
}

func (v *metaModelValidation) milestonesUnlockRecordedInvariants() {
	for _, ms := range v.model.Milestones {
		seen := map[string]bool{}
		for _, id := range ms.Unlocks {
			if seen[id] {
				v.failf("milestones %q: unlocks %q twice", ms.Key, id)
			}
			seen[id] = true
			inv, ok := v.invariants[id]
			if !ok {
				v.failf("milestones %q: unlocks %q, which is not a recorded invariant", ms.Key, id)
				continue
			}
			if inv.Enforcement.Needs != ms.Key {
				v.failf("milestones %q: unlocks %q, but that invariant needs %q", ms.Key, id, inv.Enforcement.Needs)
			}
		}
	}
}
