package application_test

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// agentsFixture extends contextFixture with an extension Rule bound to
// zone "m" and a repository-scoped extension Rule, so every block
// section has material.
func agentsFixture(t *testing.T) rule.Configured {
	t.Helper()
	cfg := contextFixture(t)
	scope, err := rule.ZoneScope([]rule.ZoneName{"m"})
	if err != nil {
		t.Fatalf("ZoneScope: %v", err)
	}
	bound, err := rule.New(rule.Spec{
		ID:        "t/p:m/technology-free",
		Rationale: "Keep transport choices outside the model.",
		Type:      rule.TypeExtension,
		Severity:  "warning",
		Params:    rule.ExtensionParams{Uses: "forbid-content", With: map[string]any{"pattern": `"net/http"`}},
		Scope:     scope,
	})
	if err != nil {
		t.Fatalf("rule.New: %v", err)
	}
	repo, err := rule.RepositoryScope()
	if err != nil {
		t.Fatalf("RepositoryScope: %v", err)
	}
	isolation, err := rule.New(rule.Spec{
		ID:     "t/p:fsd/slice-isolation",
		Type:   rule.TypeExtension,
		Params: rule.ExtensionParams{Uses: "fsd-slice-isolation", With: map[string]any{"layers": []any{"a", "b"}}},
		Scope:  repo,
	})
	if err != nil {
		t.Fatalf("rule.New: %v", err)
	}
	cfg.Rules = append(cfg.Rules, bound, isolation)
	return cfg
}

func recordedKnowledge(t *testing.T) *fakeKnowledge {
	t.Helper()
	lang, err := vocab.NewUbiquitousLanguage("boxoffice", "", []vocab.BoundedContext{
		{
			Name:       "catalog",
			Definition: "what is on sale",
			Aggregates: []vocab.Aggregate{{
				Name:       "Event",
				Definition: "one show",
				Identity:   "EventID",
				Entities:   []vocab.Entity{{Name: "Organizer", Definition: "whose page it is"}},
				Invariants: []vocab.Invariant{{Key: "sells-while-published", Statement: "an Event sells only while published"}},
			}},
			ValueObjects: []vocab.ValueObject{{Name: "Price", Definition: "whole cents"}},
			Events:       []vocab.DomainEvent{{Name: "EventPublished", Definition: "the draft went on sale", RaisedBy: "Event"}},
		},
		{
			Name:       "ordering",
			Definition: "the deals struck",
			Aggregates: []vocab.Aggregate{{Name: "Order", Definition: "the deal as struck", Identity: "OrderID"}},
		},
	}, []vocab.ContextRelation{{From: "catalog", To: "ordering", Kind: vocab.RelationConformist}})
	if err != nil {
		t.Fatalf("NewUbiquitousLanguage: %v", err)
	}
	return &fakeKnowledge{lang: lang, found: true}
}

type fakeExtensionInventory struct {
	rules []application.RegisteredExtensionRule
}

func (f fakeExtensionInventory) RegisteredExtensionRules() ([]application.RegisteredExtensionRule, error) {
	return f.rules, nil
}

type fakePublisher struct {
	installed string
}

func (f *fakePublisher) Install(block string) (bool, string, error) {
	f.installed = block
	return true, "AGENTS.md", nil
}

func TestPublishAgentsContextRendersAndInstalls(t *testing.T) {
	inventory := fakeExtensionInventory{rules: []application.RegisteredExtensionRule{
		{Name: "forbid-content", Source: ".arclint/extensions/local.ts"},
		{Name: "fsd-slice-isolation", Source: ".arclint/extensions/local.ts"},
	}}
	publisher := &fakePublisher{}
	publish, err := application.NewPublishAgentsContext(
		fakeRepository{agentsFixture(t)}, recordedKnowledge(t), inventory, publisher)
	if err != nil {
		t.Fatalf("NewPublishAgentsContext: %v", err)
	}
	block, err := publish.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	for _, want := range []string{
		application.AgentsBegin, application.AgentsEnd,
		"5 rules over languages [go]",
		"### Workflow",
		"IMPORTANT: work in this order on every change.",
		"1. Run `arclint context <paths>` on the files you will read or change, before opening them.",
		"do not learn the architecture by reading file after file",
		"2. Before editing, state in the project's language:\n   - This change does ___.\n   - These decisions belong to ___.\n   - The caller uses ___.\n   - Existing behavior ___ must remain intact.",
		"3. If the change introduces or changes a meaning, record it in `domain.arclint.yaml` before writing code, using the domain-librarian skill.",
		"4. Implement one complete behavior path through the zones `arclint context` reported",
		"### Commands",
		"- `arclint agents skill`: write the domain-librarian skill to `.agents/skills/domain-librarian/` when your harness lacks it",
		"### The recorded domain",
		"2 contexts, 2 aggregates, 1 value objects, 1 invariants (domain.arclint.yaml).",
		"- **catalog**: aggregates Event (Organizer); value objects Price; events EventPublished",
		"- **ordering**: aggregates Order",
		"Relations: catalog → ordering (conformist). Full text: `arclint domain`.",
		"### Zones and their rules",
		"- **m**: test zone (paths m/**)",
		"  - imports no other zone; external imports forbidden",
		"  - snake: file names use snake_case",
		`  - technology-free (warning): satisfies extension rule "forbid-content" (pattern: "net/http")`,
		`Rationale: Keep transport choices outside the model.`,
		"### Repository-wide rules",
		`- deps/protected-m: Zone "m" is imported by no other Zone`,
		`- fsd/slice-isolation: satisfies extension rule "fsd-slice-isolation" (layers: [a, b])`,
		"### Extension rules",
		"`.arclint/extensions/local.ts` default-exports the rule definitions: forbid-content, fsd-slice-isolation.",
	} {
		if !strings.Contains(block, want) {
			t.Errorf("block lacks %q:\n%s", want, block)
		}
	}
	// The workflow comes first, then the commands, then what the
	// repository records and enforces.
	workflowAt := strings.Index(block, "### Workflow")
	commandsAt := strings.Index(block, "### Commands")
	domainAt := strings.Index(block, "### The recorded domain")
	zonesAt := strings.Index(block, "### Zones and their rules")
	if !(workflowAt >= 0 && workflowAt < commandsAt && commandsAt < domainAt && domainAt < zonesAt) {
		t.Fatalf("section order wrong: workflow at %d, commands at %d, recorded domain at %d, zones at %d:\n%s",
			workflowAt, commandsAt, domainAt, zonesAt, block)
	}
	// The workflow is AgentWorkflow, step for step, and names only commands
	// the surface documents, so an agent following it never runs a command
	// arclint does not ship.
	workflow := block[workflowAt:commandsAt]
	for index, step := range application.AgentWorkflow("domain.arclint.yaml") {
		if !strings.Contains(workflow, fmt.Sprintf("%d. %s\n", index+1, step)) {
			t.Errorf("workflow lacks step %d %q:\n%s", index+1, step, workflow)
		}
	}
	for _, named := range regexp.MustCompile("`arclint ([a-z ]+?)[ .`]").FindAllStringSubmatch(workflow, -1) {
		if !slices.ContainsFunc(application.AgentCommandSurface(), func(c application.AgentCommandDoc) bool {
			return c.Command == named[1]
		}) {
			t.Errorf("workflow names `arclint %s`, which the command surface does not document", named[1])
		}
	}
	if !strings.Contains(workflow, "`arclint context <paths>`") || !strings.Contains(workflow, "`arclint check .`") {
		t.Errorf("workflow must start from `arclint context` and gate on `arclint check .`:\n%s", workflow)
	}
	// The command surface renders every entry as an invocable bullet.
	for _, c := range application.AgentCommandSurface() {
		if !strings.Contains(block, "- `arclint "+c.Usage+"`: ") {
			t.Errorf("block lacks the %q command bullet:\n%s", c.Command, block)
		}
	}
	// The consumes Rule folds into the imports line, never repeats as a
	// nested rule, and the block carries no self-disclaimer.
	for _, reject := range []string{"imports no other declared Zone", "_Generated by"} {
		if strings.Contains(block, reject) {
			t.Errorf("block must not contain %q:\n%s", reject, block)
		}
	}
	if _, _, err := publish.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if publisher.installed != block {
		t.Errorf("installed block differs from rendered block")
	}
}

func TestPublishAgentsContextOmitsAbsentSections(t *testing.T) {
	cfg, _ := fixture(t, "m/ok.go")
	publish, err := application.NewPublishAgentsContext(
		fakeRepository{cfg}, emptyKnowledge(), fakeExtensionInventory{}, &fakePublisher{})
	if err != nil {
		t.Fatalf("NewPublishAgentsContext: %v", err)
	}
	block, err := publish.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	for _, reject := range []string{
		"### The recorded domain", "### Repository-wide rules", "### Extension rules",
		"Rationale:",
	} {
		if strings.Contains(block, reject) {
			t.Errorf("block must omit %q without its data:\n%s", reject, block)
		}
	}
	// The workflow and the commands are unconditional: they render even
	// without a recorded domain, and never gate on installed skill files.
	for _, want := range []string{
		"### Workflow", "### Commands",
		"### Zones and their rules", "- **m**",
	} {
		if !strings.Contains(block, want) {
			t.Errorf("block lacks %q:\n%s", want, block)
		}
	}
}

func TestPublishAgentsContextSpellsPatternRulesQualified(t *testing.T) {
	cfg := agentsFixture(t)
	shared, err := rule.ParsePatternReference("acme/layers@1.2.0")
	if err != nil {
		t.Fatalf("ParsePatternReference: %v", err)
	}
	scope, err := rule.ZoneScope([]rule.ZoneName{"m"})
	if err != nil {
		t.Fatalf("ZoneScope: %v", err)
	}
	repo, err := rule.RepositoryScope()
	if err != nil {
		t.Fatalf("RepositoryScope: %v", err)
	}
	glob, err := rule.NewGlob("m/root.go")
	if err != nil {
		t.Fatalf("NewGlob: %v", err)
	}
	bound, err := rule.New(rule.Spec{
		ID:         "acme/layers:m/has-root",
		Type:       rule.TypeStructure,
		Params:     rule.StructureParams{Require: []rule.Glob{glob}},
		Scope:      scope,
		Provenance: &shared,
	})
	if err != nil {
		t.Fatalf("rule.New: %v", err)
	}
	wide, err := rule.New(rule.Spec{
		ID:         "acme/layers:deps/acyclic",
		Type:       rule.TypeAcyclic,
		Params:     rule.AcyclicParams{},
		Scope:      repo,
		Provenance: &shared,
	})
	if err != nil {
		t.Fatalf("rule.New: %v", err)
	}
	cfg.Rules = append(cfg.Rules, bound, wide)
	publish, err := application.NewPublishAgentsContext(
		fakeRepository{cfg}, emptyKnowledge(), fakeExtensionInventory{}, &fakePublisher{})
	if err != nil {
		t.Fatalf("NewPublishAgentsContext: %v", err)
	}
	block, err := publish.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	// The Pattern is named once with its rule count, and every Rule it
	// distributes keeps the qualified id an Override or `arclint rules`
	// takes; local Rules keep their short spelling.
	for _, want := range []string{
		"Extended Patterns: `acme/layers@1.2.0` (2 rules, ids qualified `acme/layers:`).",
		"never by editing the Pattern.",
		"  - acme/layers:m/has-root: ",
		"- acme/layers:deps/acyclic: ",
		"  - snake: file names use snake_case",
		`- deps/protected-m: Zone "m" is imported by no other Zone`,
	} {
		if !strings.Contains(block, want) {
			t.Errorf("block lacks %q:\n%s", want, block)
		}
	}
	for _, reject := range []string{"  - has-root: ", "\n- deps/acyclic: "} {
		if strings.Contains(block, reject) {
			t.Errorf("a Pattern Rule must not lose its namespace (%q):\n%s", reject, block)
		}
	}
}

// The built-in Rules a recorded domain composes get their own section
// that says where they come from and how a ruleset adopts one; they
// never masquerade as repository-wide Rules the ruleset wrote.
func TestPublishAgentsContextSectionsTheBuiltInRules(t *testing.T) {
	cfg := agentsFixture(t)
	builtIn, err := rule.BuiltIn()
	if err != nil {
		t.Fatalf("BuiltIn: %v", err)
	}
	cfg.Rules = append(cfg.Rules, builtIn...)
	publish, err := application.NewPublishAgentsContext(
		fakeRepository{cfg}, recordedKnowledge(t), fakeExtensionInventory{}, &fakePublisher{})
	if err != nil {
		t.Fatalf("NewPublishAgentsContext: %v", err)
	}
	block, err := publish.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	intro := fmt.Sprintf("%d rules %s judge the recorded domain against the code; no Pattern distributes them. "+
		"Change one through an Override under its id in rules.arclint.yaml (severity, or disable with a reason).",
		len(builtIn), application.BuiltInOrigin)
	for _, want := range []string{
		fmt.Sprintf("%d rules over languages [go]", 5+len(builtIn)),
		"### Built-in rules\n\n" + intro + "\n\n- ",
		"\n- aggregate/root-declared: ",
		"\n- bounded_context/isolated: ",
		"\n- aggregate/protects-an-invariant (warning): ",
	} {
		if !strings.Contains(block, want) {
			t.Errorf("block lacks %q:\n%s", want, block)
		}
	}
	builtInAt := strings.Index(block, "### Built-in rules")
	repositoryAt := strings.Index(block, "### Repository-wide rules")
	if !(builtInAt >= 0 && builtInAt < repositoryAt) {
		t.Errorf("built-in rules must precede the repository-wide ones: %d, %d:\n%s", builtInAt, repositoryAt, block)
	}
	repository := block[repositoryAt:]
	for _, r := range builtIn {
		if strings.Contains(repository, "\n- "+r.ID().Qualified()+": ") {
			t.Errorf("built-in %s listed again under the repository-wide rules:\n%s", r.ID().Qualified(), repository)
		}
	}
}
