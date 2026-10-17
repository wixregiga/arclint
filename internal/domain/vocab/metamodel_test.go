package vocab

import (
	"regexp"
	"strings"
	"testing"
)

// The generated table is the meta-model the binary ships; it must
// satisfy every rule the meta-model states about itself.
func TestDDDValidates(t *testing.T) {
	if err := DDD().Validate(); err != nil {
		t.Fatalf("metamodel.arclint.yaml does not validate:\n%v", err)
	}
}

func TestDDDShape(t *testing.T) {
	m := DDD()
	if m.Name != "ddd" || m.Title != "Domain-Driven Design" {
		t.Fatalf("name/title = %q/%q", m.Name, m.Title)
	}
	for _, phase := range []string{"strategic", "tactical", "structural"} {
		if _, ok := m.Phase(phase); !ok {
			t.Errorf("phase %q missing", phase)
		}
	}
	for _, term := range []string{"bounded_context", "aggregate", "value_object", "invariant", "domain_isolation", "zone"} {
		if _, ok := m.Block(term); !ok {
			t.Errorf("building block %q missing", term)
		}
	}
	for _, fact := range []string{"file_tree", "imports", "declarations", "calls"} {
		if _, ok := m.Fact(fact); !ok {
			t.Errorf("fact %q missing", fact)
		}
	}
	if len(m.Invariants()) == 0 {
		t.Fatal("no block invariants")
	}
	zone, _ := m.Block("zone")
	if len(zone.Definition.Sources) != 0 {
		t.Errorf("zone is arclint's own term and cites nothing; got %v", zone.Definition.Sources)
	}
	if _, ok := m.Block("layered_architecture"); ok {
		t.Error("the meta-model demands isolation, not a layered architecture; layered_architecture must not be a block")
	}
}

func TestDDDReturnsFreshValues(t *testing.T) {
	first := DDD()
	first.Blocks[0].Term = "mutated"
	first.Blocks[0].Invariants = nil
	first.Works[0].Authors[0] = "nobody"
	second := DDD()
	if second.Blocks[0].Term == "mutated" || second.Works[0].Authors[0] == "nobody" {
		t.Fatal("DDD shares its slices between calls")
	}
}

func TestDDDLookupsByIdentity(t *testing.T) {
	m := DDD()
	inv, ok := m.Invariant("aggregate/one-root")
	if !ok {
		t.Fatal("aggregate/one-root missing")
	}
	if inv.Term() != "aggregate" {
		t.Errorf("Term() = %q", inv.Term())
	}
	if inv.Level() != LevelRecording {
		t.Errorf("aggregate/one-root reads no fact and is evaluated at recording; Level() = %q", inv.Level())
	}
	if _, ok := m.Invariant("aggregate/no-such-rule"); ok {
		t.Error("unknown invariant found")
	}
	if _, ok := m.Work("evans2015"); !ok {
		t.Error("evans2015 missing from the bibliography")
	}
	if _, ok := m.Milestone("field-mutability"); !ok {
		t.Error("field-mutability milestone missing")
	}
}

func TestBlockInvariantLevel(t *testing.T) {
	cases := []struct {
		name string
		e    Enforcement
		want EnforcementLevel
	}{
		{"loader reads the file alone", Enforcement{By: EvaluatorLoader}, LevelRecording},
		{"domain evaluator is a check", Enforcement{By: EvaluatorDomain, Facts: []string{"declarations"}}, LevelCheck},
		{"domain evaluator without facts is still a check", Enforcement{By: EvaluatorDomain}, LevelCheck},
		{"planned is a check", Enforcement{By: EvaluatorPlanned}, LevelCheck},
		{"a milestone outranks facts", Enforcement{Needs: "field-types", Facts: []string{"declarations"}}, LevelMilestone},
		{"a milestone without facts", Enforcement{Needs: "transaction-facts"}, LevelMilestone},
	}
	for _, c := range cases {
		if got := (BlockInvariant{Enforcement: c.e}).Level(); got != c.want {
			t.Errorf("%s: Level() = %q, want %q", c.name, got, c.want)
		}
	}
}

// Every level the meta-model distinguishes is present in the shipped
// table, and each is consistent with the enforcement that implies it.
func TestDDDLevelsAreConsistent(t *testing.T) {
	m := DDD()
	seen := map[EnforcementLevel]int{}
	for _, inv := range m.Invariants() {
		level := inv.Level()
		seen[level]++
		e := inv.Enforcement
		switch level {
		case LevelRecording:
			if len(e.Facts) != 0 || e.Needs != "" || e.By != EvaluatorLoader {
				t.Errorf("%s: recording-level invariant with facts %v, needs %q, by %q", inv.ID, e.Facts, e.Needs, e.By)
			}
		case LevelCheck:
			if e.Needs != "" || e.By == "" || e.By == EvaluatorLoader {
				t.Errorf("%s: check-level invariant with needs %q, by %q", inv.ID, e.Needs, e.By)
			}
		case LevelMilestone:
			if e.Needs == "" || e.By != "" {
				t.Errorf("%s: milestone invariant with needs %q, by %q", inv.ID, e.Needs, e.By)
			}
		}
	}
	for _, level := range []EnforcementLevel{LevelRecording, LevelCheck, LevelMilestone} {
		if seen[level] == 0 {
			t.Errorf("no invariant at level %q", level)
		}
	}
}

func TestDDDWarningInvariantsNeverReject(t *testing.T) {
	m := DDD()
	var warnings []string
	for _, inv := range m.Invariants() {
		if inv.Enforcement.Severity == "warning" {
			warnings = append(warnings, inv.ID)
		}
	}
	want := []string{"aggregate/protects-an-invariant", "aggregate/commands-named-for-behavior"}
	if strings.Join(warnings, ",") != strings.Join(want, ",") {
		t.Fatalf("warning-severity invariants = %v, want %v", warnings, want)
	}
}

func TestDDDFundamentalGuidanceIsMarked(t *testing.T) {
	m := DDD()
	for _, term := range []string{"aggregate", "domain_isolation", "invariant", "ubiquitous_language"} {
		b, ok := m.Block(term)
		if !ok {
			t.Fatalf("block %q missing", term)
		}
		marked := 0
		for _, g := range b.Guidance {
			if g.Fundamental {
				marked++
			}
		}
		if marked == 0 {
			t.Errorf("%s: no guidance is marked fundamental; the pattern's own directive must be", term)
		}
	}
}

// validModel is the smallest meta-model that validates; each rejection
// case below breaks exactly one thing in a copy of it.
func validModel() MetaModel {
	cite := []Citation{{Work: "evans2015", Page: "16", Section: "Aggregates"}}
	return MetaModel{
		Name:        "ddd",
		Title:       "Domain-Driven Design",
		Description: "The vocabulary.",
		Works: []Work{
			{
				Key: "evans2015", Title: "Reference", Authors: []string{"Eric Evans"},
				Year: 2015, Tier: TierCanonical, Verification: VerifiedText,
			},
			{
				Key: "fowler-aggregate", Title: "DDD_Aggregate", Authors: []string{"Martin Fowler"},
				Date: "2013-04-23", Tier: TierPublished, Verification: VerifiedFetched,
			},
			{
				Key: "a-blog", Title: "A Blog Post", Authors: []string{"A Practitioner"},
				Year: 2020, Tier: TierPractitioner, Verification: VerifiedFetched,
			},
		},
		Facts: []FactDefinition{{Name: "declarations", Definition: "Declarations.", Languages: Languages{All: true}}},
		Milestones: []Milestone{{
			Key: "field-types", Needs: "Field types.", Unlocks: []string{"aggregate/references-by-identity"},
		}},
		Phases: []Phase{
			{Name: "tactical", Title: "Tactical design", Definition: Meaning{Text: "Blocks.", Sources: cite}},
			{Name: "structural", Title: "Structural vocabulary", Definition: Meaning{Text: "arclint's own terms."}},
		},
		Blocks: []BuildingBlock{
			{
				Term: "aggregate", Title: "Aggregate", Phase: "tactical",
				Definition: Meaning{Text: "A cluster.", Sources: cite},
				Records: Recording{
					Section:    "aggregates",
					Properties: []Property{{Name: "identity", Required: true, Definition: "The root's identity."}},
					Kinds:      []KindDefinition{{Name: "plain", Definition: "Plain.", Sources: cite}},
				},
				Invariants: []BlockInvariant{
					{
						ID: "aggregate/identity-recorded", Statement: "Names its identity.", Sources: cite,
						Enforcement: Enforcement{By: EvaluatorLoader, Languages: Languages{All: true}, Severity: "error"},
					},
					{
						ID: "aggregate/root-declared", Statement: "The root is declared.", Sources: cite,
						Enforcement: Enforcement{
							By: EvaluatorDomain, Facts: []string{"declarations"},
							Languages: Languages{Names: []string{"go"}}, Severity: "warning",
						},
					},
					{
						ID: "aggregate/references-by-identity", Statement: "Holds identities.", Sources: cite,
						Enforcement: Enforcement{Needs: "field-types", Facts: []string{"declarations"}, Severity: "error"},
					},
				},
				Guidance: []Guidance{{Text: "Keep it small.", Fundamental: true, Sources: cite}},
			},
			{
				Term: "zone", Title: "Zone", Phase: "structural",
				Definition: Meaning{Text: "A partition."},
				Records:    Recording{Section: "none"},
			},
		},
		Declined: []DeclinedRule{
			{
				ID: "aggregate/required-factory", Statement: "Every aggregate names a factory.",
				Reason: "Only complicated creation calls for one.", Sources: cite,
			},
			{
				ID: "aggregate/waived-when-missing", Statement: "A waiver silences a missing rule.",
				Reason: "A warning does not fail.", Decision: DecisionIssues + "129",
			},
		},
	}
}

// A tier 3 work may illustrate a declined rule beside a tier 1 or 2
// work that names a page.
func TestValidateAcceptsIllustratingBlog(t *testing.T) {
	m := validModel()
	m.Declined[0].Sources = append(m.Declined[0].Sources, Citation{Work: "a-blog"})
	if err := m.Validate(); err != nil {
		t.Fatalf("rejected: %v", err)
	}
}

func TestValidateAcceptsMinimalModel(t *testing.T) {
	if err := validModel().Validate(); err != nil {
		t.Fatalf("minimal model rejected: %v", err)
	}
}

func TestValidateRejects(t *testing.T) {
	cases := []struct {
		name  string
		mut   func(m *MetaModel)
		wants string
	}{
		{"unlisted work", func(m *MetaModel) { m.Blocks[0].Invariants[0].Sources[0].Work = "nobody" }, `cites work "nobody"`},
		{"duplicate term", func(m *MetaModel) { m.Blocks[1].Term = "aggregate" }, `term "aggregate" recorded twice`},
		{"id off its block", func(m *MetaModel) { m.Blocks[0].Invariants[0].ID = "entity/identity-recorded" }, `prefixed by its block's term "aggregate"`},
		{"id not kebab", func(m *MetaModel) { m.Blocks[0].Invariants[0].ID = "aggregate/Identity_Recorded" }, `not lowercase kebab case`},
		{"duplicate id", func(m *MetaModel) { m.Blocks[0].Invariants[1].ID = "aggregate/identity-recorded" }, `id recorded twice`},
		{"unknown phase", func(m *MetaModel) { m.Blocks[0].Phase = "mystical" }, `phase "mystical"`},
		{"uncited block", func(m *MetaModel) { m.Blocks[0].Definition.Sources = nil }, `must cite at least one work`},
		{"cited structural block", func(m *MetaModel) {
			m.Blocks[1].Definition.Sources = []Citation{{Work: "evans2015"}}
		}, `structural term cites nothing`},
		{"uncited invariant", func(m *MetaModel) { m.Blocks[0].Invariants[0].Sources = nil }, `invariant must cite`},
		{"uncited guidance", func(m *MetaModel) { m.Blocks[0].Guidance[0].Sources = nil }, `guidance must cite`},
		{"bad severity", func(m *MetaModel) { m.Blocks[0].Invariants[0].Enforcement.Severity = "info" }, `severity "info"`},
		{"unpublished evaluator", func(m *MetaModel) { m.Blocks[0].Invariants[0].Enforcement.By = "oracle" }, `evaluator "oracle"`},
		{"loader with facts", func(m *MetaModel) {
			m.Blocks[0].Invariants[0].Enforcement.Facts = []string{"declarations"}
		}, `loader reads the domain file alone`},
		{"unobserved fact", func(m *MetaModel) {
			m.Blocks[0].Invariants[1].Enforcement.Facts = []string{"field_types"}
		}, `fact "field_types"`},
		{"no languages", func(m *MetaModel) { m.Blocks[0].Invariants[0].Enforcement.Languages = Languages{} }, `languages must be all`},
		{"all and names", func(m *MetaModel) {
			m.Blocks[0].Invariants[0].Enforcement.Languages = Languages{All: true, Names: []string{"go"}}
		}, `all and also names`},
		{"milestone with evaluator", func(m *MetaModel) {
			m.Blocks[0].Invariants[2].Enforcement.By = EvaluatorPlanned
		}, `names no evaluator until the milestone lands`},
		{"unknown milestone", func(m *MetaModel) { m.Blocks[0].Invariants[2].Enforcement.Needs = "telepathy" }, `milestone "telepathy"`},
		{"milestone forgets the invariant", func(m *MetaModel) { m.Milestones[0].Unlocks = []string{"aggregate/root-declared"} }, `does not list it under unlocks`},
		{"milestone unlocks a stranger", func(m *MetaModel) {
			m.Milestones[0].Unlocks = append(m.Milestones[0].Unlocks, "aggregate/nothing")
		}, `unlocks "aggregate/nothing"`},
		{"duplicate property", func(m *MetaModel) {
			m.Blocks[0].Records.Properties = append(m.Blocks[0].Records.Properties, Property{Name: "identity", Definition: "Again."})
		}, `property "identity" recorded twice`},
		{"duplicate kind", func(m *MetaModel) {
			m.Blocks[0].Records.Kinds = append(m.Blocks[0].Records.Kinds, m.Blocks[0].Records.Kinds[0])
		}, `kind "plain" recorded twice`},
		{"missing section", func(m *MetaModel) { m.Blocks[0].Records.Section = "" }, `records.section`},
		{"work without authors", func(m *MetaModel) { m.Works[0].Authors = nil }, `at least one author`},
		{"work verification", func(m *MetaModel) { m.Works[0].Verification = "trusted" }, `verification "trusted"`},
		{"empty statement", func(m *MetaModel) { m.Blocks[0].Invariants[0].Statement = " " }, `statement must be non-empty`},
		{"Stack Overflow work", func(m *MetaModel) {
			m.Works[2].URL = "https://stackoverflow.com/questions/1/aggregates"
		}, `works "a-blog": stackoverflow.com is an excluded source and is never cited`},
		{"Stack Exchange work", func(m *MetaModel) {
			m.Works[2].URL = "https://softwareengineering.StackExchange.com/q/2"
		}, `softwareengineering.stackexchange.com is an excluded source`},
		{"Goodreads work", func(m *MetaModel) {
			m.Works[2].URL = "https://www.goodreads.com/quotes/3"
		}, `www.goodreads.com is an excluded source`},
		{"fully qualified Stack Overflow host", func(m *MetaModel) {
			m.Works[2].URL = "https://stackoverflow.com./questions/123"
		}, `works "a-blog": stackoverflow.com is an excluded source`},
		{"fully qualified Goodreads host", func(m *MetaModel) {
			m.Works[2].URL = "https://www.goodreads.com./quotes/123"
		}, `www.goodreads.com is an excluded source`},
		{"Super User work", func(m *MetaModel) { m.Works[2].URL = "https://superuser.com/questions/123" }, `superuser.com is an excluded source`},
		{"Server Fault work", func(m *MetaModel) { m.Works[2].URL = "https://serverfault.com/questions/123" }, `serverfault.com is an excluded source`},
		{"Ask Ubuntu work", func(m *MetaModel) { m.Works[2].URL = "https://askubuntu.com/questions/123" }, `askubuntu.com is an excluded source`},
		{"MathOverflow work", func(m *MetaModel) { m.Works[2].URL = "https://mathoverflow.net/questions/123" }, `mathoverflow.net is an excluded source`},
		{"relative work URL", func(m *MetaModel) { m.Works[2].URL = "posts/aggregates" }, `url "posts/aggregates" is not an absolute http or https URL`},
		{"enforced invariant cites only tier 3", func(m *MetaModel) {
			m.Blocks[0].Invariants[1].Sources = []Citation{{Work: "a-blog", Illustrative: true}}
		}, `invariant aggregate/root-declared: an invariant is a rule, and it must cite at least one tier 1 work`},
		{"enforced invariant cites only tier 2", func(m *MetaModel) {
			m.Blocks[0].Invariants[1].Sources = []Citation{{Work: "fowler-aggregate"}}
		}, `invariant aggregate/root-declared: an invariant is a rule, and it must cite at least one tier 1 work`},
		{"invariant grounded only by an illustrative tier 1 citation", func(m *MetaModel) {
			m.Blocks[0].Invariants[0].Sources = []Citation{{Work: "evans2015", Page: "16", Illustrative: true}}
		}, `invariant aggregate/identity-recorded: an invariant is a rule, and it must cite at least one tier 1 work`},
		{"milestone invariant cites only tier 2", func(m *MetaModel) {
			m.Blocks[0].Invariants[2].Sources = []Citation{{Work: "fowler-aggregate"}}
		}, `invariant aggregate/references-by-identity: an invariant is a rule`},
		{"tier 3 citation not marked illustrative", func(m *MetaModel) {
			m.Blocks[0].Guidance[0].Sources = []Citation{{Work: "evans2015", Page: "16"}, {Work: "a-blog"}}
		}, `cites tier 3 work "a-blog" without marking the citation illustrative`},
		{"fundamental guidance cites only tier 2", func(m *MetaModel) {
			m.Blocks[0].Guidance[0].Sources = []Citation{{Work: "fowler-aggregate"}}
		}, `guidance 1: a fundamental guidance must cite at least one tier 1 work`},
		{"guidance cites only an illustration", func(m *MetaModel) {
			m.Blocks[0].Guidance = append(m.Blocks[0].Guidance, Guidance{
				Text: "An example.", Sources: []Citation{{Work: "a-blog", Illustrative: true}},
			})
		}, `guidance 2: guidance must cite at least one tier 1 or tier 2 work`},
		{"definition cites only an illustration", func(m *MetaModel) {
			m.Blocks[0].Definition.Sources = []Citation{{Work: "a-blog", Illustrative: true}}
		}, `building_blocks aggregate: definition must cite at least one tier 1 or tier 2 work`},
		{"kind cites only an illustration", func(m *MetaModel) {
			m.Blocks[0].Records.Kinds[0].Sources = []Citation{{Work: "evans2015", Illustrative: true}}
		}, `kind plain: definition must cite at least one tier 1 or tier 2 work`},
		{"convention that quotes nothing of the entry", func(m *MetaModel) {
			m.Blocks[0].Invariants[0].Conventions = []string{"its identity is a UUID"}
		}, `convention label "its identity is a UUID" does not quote the entry's text`},
		{"empty convention", func(m *MetaModel) { m.Blocks[0].Guidance[0].Conventions = []string{" "} }, `a convention label must quote the words`},
		{"convention recorded twice", func(m *MetaModel) {
			m.Blocks[0].Definition.Conventions = []string{"cluster", "cluster"}
		}, `convention label "cluster" recorded twice`},
		{"limitation off the domain evaluator", func(m *MetaModel) {
			m.Blocks[0].Invariants[0].Enforcement.Limitations = []string{"a thrown error is not seen"}
		}, `evaluator "loader" runs none`},
		{"declined off every block", func(m *MetaModel) { m.Declined[0].ID = "nowhere/required-factory" }, `prefixed by a building block's term`},
		{"declined without a slash", func(m *MetaModel) { m.Declined[0].ID = "aggregate" }, `prefixed by a building block's term`},
		{"declined name not kebab", func(m *MetaModel) { m.Declined[0].ID = "aggregate/Required_Factory" }, `not lowercase kebab case`},
		{"declined names an invariant", func(m *MetaModel) { m.Declined[0].ID = "aggregate/identity-recorded" }, `names an invariant arclint enforces`},
		{"declined twice", func(m *MetaModel) { m.Declined[1].ID = m.Declined[0].ID }, `id recorded twice`},
		{"declined without statement", func(m *MetaModel) { m.Declined[0].Statement = " " }, `statement must say what is not enforced`},
		{"declined without reason", func(m *MetaModel) { m.Declined[0].Reason = "" }, `reason must say why`},
		{"declined without basis", func(m *MetaModel) { m.Declined[1].Decision = "" }, `a basis is required`},
		{"declined on two bases", func(m *MetaModel) { m.Declined[0].Decision = DecisionIssues + "1" }, `not both`},
		{"declined literature without a page", func(m *MetaModel) {
			m.Declined[0].Sources = []Citation{{Work: "evans2015", Section: "Factories"}}
		}, `name a page`},
		{"declined literature unlisted", func(m *MetaModel) {
			m.Declined[0].Sources = []Citation{{Work: "nobody", Page: "1"}}
		}, `cites work "nobody"`},
		{"declined literature a blog alone", func(m *MetaModel) {
			m.Declined[0].Sources = []Citation{{Work: "a-blog", Page: "1"}}
		}, `a tier 3 work may only illustrate`},
		{"work without tier", func(m *MetaModel) { m.Works[0].Tier = 0 }, `tier 0 is not 1, 2, or 3`},
		{"work beyond tier 3", func(m *MetaModel) { m.Works[1].Tier = 4 }, `tier 4 is not 1, 2, or 3`},
		{"declined decision elsewhere", func(m *MetaModel) {
			m.Declined[1].Decision = "https://example.com/issues/129"
		}, `not a link to an arclint issue`},
		{"declined decision not an issue number", func(m *MetaModel) {
			m.Declined[1].Decision = DecisionIssues + "129#comment"
		}, `not a link to an arclint issue`},
		{"empty limitation", func(m *MetaModel) {
			m.Blocks[0].Invariants[1].Enforcement.By = EvaluatorDomain
			m.Blocks[0].Invariants[1].Enforcement.Limitations = []string{" "}
		}, `a limitation is empty`},
	}
	for _, c := range cases {
		m := validModel()
		c.mut(&m)
		err := m.Validate()
		if err == nil {
			t.Errorf("%s: accepted", c.name)
			continue
		}
		if !strings.Contains(err.Error(), c.wants) {
			t.Errorf("%s: error %q does not mention %q", c.name, err, c.wants)
		}
	}
}

// Tier 2 grounds guidance, a tier 3 work may illustrate anything when the
// citation says so, and a convention label that quotes its entry is
// accepted.
func TestValidateAcceptsGroundedGuidanceAndLabelledConventions(t *testing.T) {
	m := validModel()
	b := &m.Blocks[0]
	b.Guidance = append(b.Guidance, Guidance{
		Text:        "Keep the root named for the aggregate.",
		Sources:     []Citation{{Work: "fowler-aggregate"}, {Work: "a-blog", Illustrative: true}},
		Conventions: []string{"named for the aggregate"},
	})
	b.Invariants[1].Sources = []Citation{{Work: "evans2015", Page: "16"}, {Work: "a-blog", Illustrative: true}}
	b.Invariants[1].Conventions = []string{"is declared"}
	b.Definition.Conventions = []string{"A cluster"}
	if err := m.Validate(); err != nil {
		t.Fatalf("rejected: %v", err)
	}
}

func TestReferenceShowsTierAndVerification(t *testing.T) {
	carried := Work{Title: "Implementing Domain-Driven Design", Authors: []string{"Vaughn Vernon"}, Year: 2013, Tier: TierCanonical, Verification: VerifiedCarried}
	read := Work{Title: "A Blog Post", Authors: []string{"A Practitioner"}, Year: 2020, URL: "https://example.com/post", Tier: TierPractitioner, Verification: VerifiedFetched}
	cases := []struct {
		ref  Reference
		want string
	}{
		{Reference{Work: carried, Chapter: 10, Page: "359"}, "Vaughn Vernon, Implementing Domain-Driven Design (2013), ch. 10, p. 359 (tier 1, unverified page)"},
		{Reference{Work: carried, Chapter: 4, Section: "Isolating the Domain"}, "Vaughn Vernon, Implementing Domain-Driven Design (2013), ch. 4, Isolating the Domain (tier 1, unverified)"},
		{Reference{Work: read, Illustrative: true}, "A Practitioner, A Blog Post (2020) <https://example.com/post> (tier 3, illustrative, read)"},
	}
	for _, c := range cases {
		if got := c.ref.String(); got != c.want {
			t.Errorf("got  %q\nwant %q", got, c.want)
		}
	}
}

// Every carried citation the meta-model makes is shown as unverified,
// and every other as read, so no carried page reaches a reader looking
// like a checked one.
func TestDDDReferencesShowTheirVerification(t *testing.T) {
	m := DDD()
	var cites []Citation
	for _, p := range m.Phases {
		cites = append(cites, p.Definition.Sources...)
	}
	for _, b := range m.Blocks {
		cites = append(cites, b.Definition.Sources...)
		for _, k := range b.Records.Kinds {
			cites = append(cites, k.Sources...)
		}
		for _, inv := range b.Invariants {
			cites = append(cites, inv.Sources...)
		}
		for _, g := range b.Guidance {
			cites = append(cites, g.Sources...)
		}
	}
	carried := 0
	for _, r := range m.References(cites) {
		s := r.String()
		if r.Work.Verification == VerifiedCarried {
			carried++
			if !strings.Contains(s, "unverified") {
				t.Errorf("carried reference shown as checked: %s", s)
			}
			continue
		}
		if !strings.HasSuffix(s, ", read)") {
			t.Errorf("checked reference not shown as read: %s", s)
		}
	}
	if carried == 0 {
		t.Fatal("the meta-model cites no carried page; the check above proves nothing")
	}
}

func TestValidateReportsEveryFailure(t *testing.T) {
	m := validModel()
	m.Blocks[0].Invariants[0].Sources[0].Work = "nobody"
	m.Blocks[0].Phase = "mystical"
	m.Works[0].Authors = nil
	err := m.Validate()
	if err == nil {
		t.Fatal("accepted")
	}
	for _, want := range []string{`"nobody"`, `"mystical"`, "at least one author"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("joined error lacks %q:\n%v", want, err)
		}
	}
}

// A declined rule that names a block's rule, such as factory/declared in
// its reason, names one the meta-model enforces, so a renamed invariant
// cannot leave a reason pointing at nothing.
func TestDDDDeclinedReasonsNameRecordedInvariants(t *testing.T) {
	m := DDD()
	if len(m.Declined) == 0 {
		t.Fatal("the meta-model records no declined rule")
	}
	ruleID := regexp.MustCompile(`\b([a-z_]+)/([a-z][a-z0-9-]*)\b`)
	for _, d := range m.Declined {
		for _, match := range ruleID.FindAllStringSubmatch(d.Reason, -1) {
			if _, ok := m.Block(match[1]); !ok {
				continue
			}
			if _, ok := m.Invariant(match[0]); !ok {
				t.Errorf("declined %s: reason names %s, which is not a recorded invariant", d.ID, match[0])
			}
		}
	}
}

// The factory entry is the one the issue asks the librarian to cite:
// asked to require a factory for every aggregate, it answers with Evans,
// DDD Reference 2015, p. 18.
func TestDDDDeclinesFactoryForEveryAggregate(t *testing.T) {
	m := DDD()
	for _, d := range m.Declined {
		if d.ID != "factory/required-for-every-aggregate" {
			continue
		}
		if d.Term() != "factory" {
			t.Errorf("term = %q, want factory", d.Term())
		}
		for _, c := range d.Sources {
			if c.Work == "evans2015" && c.Page == "18" {
				return
			}
		}
		t.Fatalf("sources = %v, want Evans 2015 p. 18", d.Sources)
	}
	t.Fatal("factory/required-for-every-aggregate is not declined")
}

// Asked to forbid function parameters that carry out effects in a domain
// operation, the librarian answers with the owner decision of issue #169:
// the rule belongs in a Pattern, and the design review flags the shape.
func TestDDDDeclinesEffectParametersInDomainOperations(t *testing.T) {
	m := DDD()
	for _, d := range m.Declined {
		if d.ID != "domain_service/no-effect-parameters" {
			continue
		}
		if d.Term() != "domain_service" {
			t.Errorf("term = %q, want domain_service", d.Term())
		}
		if want := DecisionIssues + "169"; d.Decision != want {
			t.Errorf("decision = %q, want %q", d.Decision, want)
		}
		if len(d.Sources) != 0 {
			t.Errorf("sources = %v; an owner decision is the only basis", d.Sources)
		}
		if !strings.Contains(d.Reason, "Pattern") {
			t.Errorf("reason = %q, want it to send the rule to a Pattern", d.Reason)
		}
		return
	}
	t.Fatal("domain_service/no-effect-parameters is not declined")
}

// The canonical works stay in the bibliography at tier 1: a canonical
// reference is never removed or demoted.
func TestDDDKeepsCanonicalWorksAtTierOne(t *testing.T) {
	m := DDD()
	for _, key := range []string{
		"evans2003", "evans2015", "vernon2013", "evans-fowler-1997",
		"vernon-effective-aggregate-design-1", "vernon-effective-aggregate-design-2",
		"vernon-iddd-ch1", "vernon-iddd-sample-pages", "vernon-iddd-samples",
	} {
		w, ok := m.Work(key)
		if !ok {
			t.Errorf("canonical work %s is not in the bibliography", key)
			continue
		}
		if w.Tier != TierCanonical {
			t.Errorf("canonical work %s is %s, want %s", key, w.Tier.Label(), TierCanonical.Label())
		}
	}
}
