package vocab

import (
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
	if _, ok := m.Milestone("field-types"); !ok {
		t.Error("field-types milestone missing")
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
		Works: []Work{{
			Key: "evans2015", Title: "Reference", Authors: []string{"Eric Evans"},
			Year: 2015, Verification: VerifiedText,
		}},
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
