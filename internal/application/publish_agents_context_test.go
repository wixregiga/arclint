package application_test

import (
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// agentsFixture extends contextFixture with an extension Rule bound to
// module "m" and a repository-scoped extension Rule, so every block
// section has material.
func agentsFixture(t *testing.T) rule.Configured {
	t.Helper()
	cfg := contextFixture(t)
	scope, err := rule.ModuleApplicability([]rule.ModuleName{"m"})
	if err != nil {
		t.Fatalf("ModuleApplicability: %v", err)
	}
	bound, err := rule.New(rule.Spec{
		ID:            "t:m/technology-free",
		Type:          rule.TypeExtension,
		Severity:      "warning",
		Params:        rule.ExtensionParams{Uses: "forbid-content", With: map[string]any{"pattern": `"net/http"`}},
		Applicability: scope,
	})
	if err != nil {
		t.Fatalf("rule.New: %v", err)
	}
	repo, err := rule.RepositoryApplicability()
	if err != nil {
		t.Fatalf("RepositoryApplicability: %v", err)
	}
	isolation, err := rule.New(rule.Spec{
		ID:            "t:fsd/slice-isolation",
		Type:          rule.TypeExtension,
		Params:        rule.ExtensionParams{Uses: "fsd-slice-isolation", With: map[string]any{"layers": []any{"a", "b"}}},
		Applicability: repo,
	})
	if err != nil {
		t.Fatalf("rule.New: %v", err)
	}
	cfg.Rules = append(cfg.Rules, bound, isolation)
	return cfg
}

func recordedKnowledge(t *testing.T) *fakeKnowledge {
	t.Helper()
	lang, err := vocab.NewUbiquitousLanguage([]vocab.BoundedContext{
		{
			Name: "catalog",
			Entities: []vocab.Entity{
				{Definition: vocab.Definition{Name: "Event", Definition: "one show"}, Aggregate: true},
				{Definition: vocab.Definition{Name: "Organizer", Definition: "whose page it is"}},
			},
			ValueObjects: []vocab.Definition{{Name: "Price", Definition: "whole cents"}},
			Invariants:   []vocab.Invariant{{Statement: "an Event sells only while published", Owner: "Event"}},
			Events:       []vocab.Definition{{Name: "EventPublished", Definition: "the draft went on sale"}},
		},
		{
			Name:     "ordering",
			Entities: []vocab.Entity{{Definition: vocab.Definition{Name: "Order", Definition: "the deal as struck"}, Aggregate: true}},
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
		"### Ask arclint first",
		"IMPORTANT: you MUST ask arclint before reading around.",
		"BEFORE opening source files",
		"do NOT learn the architecture by reading file after file",
		"### The recorded domain",
		"2 contexts, 2 aggregates, 1 invariants (ubiquitous-language.yaml).",
		"- **catalog**: Event [aggregate], Organizer; value objects Price; events EventPublished",
		"- **ordering**: Order [aggregate]",
		"Relations: catalog → ordering (conformist). Full text: `arclint domain`.",
		"### Modules and their rules",
		"- **m** — test module (paths m/**)",
		"  - imports no other module; external imports forbidden",
		"  - snake: file names use snake_case",
		`  - technology-free (warning): satisfies extension rule "forbid-content" (pattern: "net/http")`,
		"### Repository-wide rules",
		`- deps/protected-m: Module "m" is imported by no other Module`,
		`- fsd/slice-isolation: satisfies extension rule "fsd-slice-isolation" (layers: [a, b])`,
		"### Local extension rules",
		"`.arclint/extensions/local.ts` default-exports the rule definitions: forbid-content, fsd-slice-isolation.",
	} {
		if !strings.Contains(block, want) {
			t.Errorf("block lacks %q:\n%s", want, block)
		}
	}
	// The command surface renders every entry as an invocable bullet.
	for _, c := range application.AgentCommandSurface() {
		if !strings.Contains(block, "- `arclint "+c.Usage+"` — ") {
			t.Errorf("block lacks the %q command bullet:\n%s", c.Command, block)
		}
	}
	// The consumes Rule folds into the imports line, never repeats as a
	// nested rule, and the block carries no self-disclaimer.
	for _, reject := range []string{"imports no other declared Module", "_Generated by"} {
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
		"### The recorded domain", "### Repository-wide rules", "### Local extension rules",
	} {
		if strings.Contains(block, reject) {
			t.Errorf("block must omit %q without its data:\n%s", reject, block)
		}
	}
	for _, want := range []string{"### Ask arclint first", "### Modules and their rules", "- **m**"} {
		if !strings.Contains(block, want) {
			t.Errorf("block lacks %q:\n%s", want, block)
		}
	}
}
