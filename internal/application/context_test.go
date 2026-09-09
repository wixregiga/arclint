package application_test

import (
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/domain/rule"
)

func contextFixture(t *testing.T) rule.Configured {
	t.Helper()
	cfg, _ := fixture(t, "m/ok.go")
	emptyAllow, err := rule.NewAllowList()
	if err != nil {
		t.Fatalf("NewAllowList: %v", err)
	}
	scope, err := rule.ZoneScope([]rule.ZoneName{"m"})
	if err != nil {
		t.Fatalf("ZoneScope: %v", err)
	}
	consumes, err := rule.New(rule.Spec{
		ID:     "t/p:m/imports",
		Type:   rule.TypeConsumes,
		Params: rule.ConsumesParams{Internal: &emptyAllow, External: rule.ImportForbid},
		Scope:  scope,
	})
	if err != nil {
		t.Fatalf("rule.New: %v", err)
	}
	repo, err := rule.RepositoryScope()
	if err != nil {
		t.Fatalf("RepositoryScope: %v", err)
	}
	protected, err := rule.New(rule.Spec{
		ID:     "t/p:deps/protected-m",
		Type:   rule.TypeProtected,
		Params: rule.ProtectedParams{Zone: "m"},
		Scope:  repo,
	})
	if err != nil {
		t.Fatalf("rule.New: %v", err)
	}
	cfg.Rules = append(cfg.Rules, consumes, protected)
	return cfg
}

func TestArchitecturalContextForPath(t *testing.T) {
	uc, err := application.NewGetArchitecturalContext(fakeRepository{contextFixture(t)}, emptyKnowledge())
	if err != nil {
		t.Fatalf("NewGetArchitecturalContext: %v", err)
	}
	ctx, err := uc.Execute(application.ContextRequest{Paths: []string{"m/service.go"}})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(ctx.Zones) != 1 || ctx.Zones[0].Name != "m" {
		t.Fatalf("owning zones = %+v, want m", ctx.Zones)
	}
	policy := ctx.Zones[0]
	if !policy.InternalRestricted || len(policy.Internal) != 0 || policy.External != "forbid" {
		t.Errorf("zone policy = %+v, want restricted-empty internal and forbidden external", policy)
	}
	reasons := map[string]string{}
	for _, r := range ctx.Rules {
		reasons[r.Summary.ID] = r.Reason
	}
	if !strings.Contains(reasons["t/p:m/snake"], "Zone(s) m") {
		t.Errorf("naming reason = %q", reasons["t/p:m/snake"])
	}
	if !strings.Contains(reasons["t/p:deps/protected-m"], "protected") {
		t.Errorf("protected reason = %q", reasons["t/p:deps/protected-m"])
	}
	if _, ok := reasons["t/p:m/imports"]; !ok {
		t.Errorf("consumes rule missing from applicable rules: %v", reasons)
	}

	outside, err := uc.Execute(application.ContextRequest{Paths: []string{"elsewhere/file.go"}})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(outside.Zones) != 0 {
		t.Errorf("a path outside every zone must own no zones, got %+v", outside.Zones)
	}
}

func TestInitializeRepositoryRejectsUnknownLanguages(t *testing.T) {
	uc, err := application.NewInitializeRepository(&recordingScaffold{}, fakePatternSource{})
	if err != nil {
		t.Fatalf("NewInitializeRepository: %v", err)
	}
	if _, err := uc.Execute(application.InitializeRepositoryRequest{Languages: []string{"rust"}}); err == nil {
		t.Errorf("unsupported language must be rejected")
	}
}

func TestInitializeRepositoryRejectsUnknownPatterns(t *testing.T) {
	uc, err := application.NewInitializeRepository(&recordingScaffold{}, fakePatternSource{})
	if err != nil {
		t.Fatalf("NewInitializeRepository: %v", err)
	}
	_, err = uc.Execute(application.InitializeRepositoryRequest{Pattern: "hexagonal"})
	if err == nil {
		t.Fatal("unknown pattern must be rejected")
	}
	want := `initialize repository: pattern "hexagonal" is not one of bare`
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
	if _, err := application.NewInitializeRepository(nil); err == nil {
		t.Errorf("a missing scaffold must be rejected")
	}
	if _, err := application.NewInitializeRepository(&recordingScaffold{}, nil); err == nil {
		t.Errorf("a nil pattern source must be rejected")
	}
}

func TestInitializeRepositoryDraftsStarter(t *testing.T) {
	scaffold := &recordingScaffold{}
	uc, err := application.NewInitializeRepository(scaffold, fakePatternSource{})
	if err != nil {
		t.Fatalf("NewInitializeRepository: %v", err)
	}
	path, err := uc.Execute(application.InitializeRepositoryRequest{Languages: []string{"go", "ts"}, Force: true})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if path != "rules.arclint.yaml" || !scaffold.force {
		t.Errorf("path = %q, force = %v", path, scaffold.force)
	}
	for _, want := range []string{"runtime: [go, ts]\n", "zones:\n", "  source: \"**\"\n", "rules:\n", "  source/dependencies:\n", "    on: source\n", "      internal: []\n"} {
		if !strings.Contains(scaffold.content, want) {
			t.Errorf("starter ruleset lacks %q:\n%s", want, scaffold.content)
		}
	}
	choices, err := uc.Patterns()
	if err != nil || len(choices) != 1 || choices[0] != application.BarePattern {
		t.Errorf("Patterns = %v, %v; want only bare", choices, err)
	}
}

func TestInitializeRepositoryAdoptsPattern(t *testing.T) {
	p := patternFixture(t, "1.2.3")
	scaffold := &recordingScaffold{}
	uc, err := application.NewInitializeRepository(scaffold, fakePatternSource{patterns: []rule.Pattern{p}})
	if err != nil {
		t.Fatalf("NewInitializeRepository: %v", err)
	}
	choices, err := uc.Patterns()
	if err != nil || len(choices) != 2 || choices[1] != "arclint/ddd-flat@1.2.3" {
		t.Fatalf("Patterns = %v, %v", choices, err)
	}
	if _, err := uc.Execute(application.InitializeRepositoryRequest{Pattern: "ddd-flat"}); err != nil {
		t.Fatalf("Execute by name: %v", err)
	}
	for _, want := range []string{
		"this repository adopts arclint/ddd-flat@1.2.3.\n",
		"# https://example.test/ddd-flat\n",
		"runtime: [go]\n",
		"extends:\n  - pattern: arclint/ddd-flat@1.2.3\n",
		"      m: \"src/m/**\"\n",
		"      # unbound: <glob>\n",
		"#     arclint/ddd-flat:m/snake:\n",
	} {
		if !strings.Contains(scaffold.content, want) {
			t.Errorf("adopting ruleset lacks %q:\n%s", want, scaffold.content)
		}
	}
	if strings.Contains(scaffold.content, "\nrules:\n") {
		t.Errorf("an adopting ruleset never copies the pattern's rules:\n%s", scaffold.content)
	}
	if _, err := uc.Execute(application.InitializeRepositoryRequest{Pattern: "arclint/ddd-flat@1.2.3"}); err != nil {
		t.Errorf("Execute by reference: %v", err)
	}
	if _, err := uc.Execute(application.InitializeRepositoryRequest{Pattern: "arclint/ddd-flat@9.9.9"}); err == nil {
		t.Errorf("an unavailable version must be rejected")
	}
	newer, err := application.NewInitializeRepository(scaffold, fakePatternSource{patterns: []rule.Pattern{p, patternFixture(t, "2.0.0")}})
	if err != nil {
		t.Fatalf("NewInitializeRepository: %v", err)
	}
	if _, err := newer.Execute(application.InitializeRepositoryRequest{Pattern: "ddd-flat"}); err != nil {
		t.Fatalf("Execute by name with two versions: %v", err)
	}
	if !strings.Contains(scaffold.content, "adopts arclint/ddd-flat@2.0.0.") {
		t.Errorf("a name resolves to the highest version:\n%s", scaffold.content)
	}
	ambiguous, err := application.NewInitializeRepository(scaffold, fakePatternSource{patterns: []rule.Pattern{p, namespacedPatternFixture(t, "acme", "1.0.0")}})
	if err != nil {
		t.Fatalf("NewInitializeRepository: %v", err)
	}
	_, err = ambiguous.Execute(application.InitializeRepositoryRequest{Pattern: "ddd-flat"})
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("a name carried by two namespaces must be rejected, got %v", err)
	}
}

// patternFixture distributes the fixture's naming Rule under the
// arclint namespace with two Zones: m with a suggested path, and
// unbound with none.
func patternFixture(t *testing.T, version string) rule.Pattern {
	t.Helper()
	return namespacedPatternFixture(t, "arclint", version)
}

func namespacedPatternFixture(t *testing.T, namespace, version string) rule.Pattern {
	t.Helper()
	snake, err := rule.NewCaseSpec("snake_case")
	if err != nil {
		t.Fatalf("NewCaseSpec: %v", err)
	}
	scope, err := rule.ZoneScope([]rule.ZoneName{"m"})
	if err != nil {
		t.Fatalf("ZoneScope: %v", err)
	}
	r, err := rule.New(rule.Spec{
		ID:     namespace + "/ddd-flat:m/snake",
		Type:   rule.TypeNaming,
		Params: rule.NamingParams{Case: snake},
		Scope:  scope,
	})
	if err != nil {
		t.Fatalf("rule.New: %v", err)
	}
	glob, err := rule.NewGlob("src/m/**")
	if err != nil {
		t.Fatalf("NewGlob: %v", err)
	}
	m, err := rule.NewPatternZone("m", "The m zone.", []rule.Glob{glob})
	if err != nil {
		t.Fatalf("NewPatternZone: %v", err)
	}
	unbound, err := rule.NewPatternZone("unbound", "A zone with no suggested path.", nil)
	if err != nil {
		t.Fatalf("NewPatternZone: %v", err)
	}
	p, err := rule.NewPattern(rule.PatternSpec{
		Namespace:     namespace,
		Name:          "ddd-flat",
		Version:       version,
		Documentation: "https://example.test/ddd-flat",
		Coverage:      []rule.Language{rule.LanguageGo},
		Zones:         []rule.PatternZone{m, unbound},
		Rules:         []rule.Rule{r},
	})
	if err != nil {
		t.Fatalf("NewPattern: %v", err)
	}
	return p
}

type recordingScaffold struct {
	content string
	force   bool
}

func (s *recordingScaffold) Write(content string, force bool) (string, error) {
	s.content, s.force = content, force
	return "rules.arclint.yaml", nil
}

func TestArchitecturalContextWorksite(t *testing.T) {
	uc, err := application.NewGetArchitecturalContext(fakeRepository{contextFixture(t)}, emptyKnowledge())
	if err != nil {
		t.Fatalf("NewGetArchitecturalContext: %v", err)
	}
	ctx, err := uc.Execute(application.ContextRequest{
		Paths: []string{"m/service.go", "m/other.go"},
		Zones: []string{"m"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(ctx.Paths) != 2 || ctx.Paths[0].Zones[0] != "m" || ctx.Paths[1].Zones[0] != "m" {
		t.Errorf("path bindings = %+v", ctx.Paths)
	}
	// The zone card appears once although three scope parts share it.
	if len(ctx.Zones) != 1 || ctx.Zones[0].Name != "m" {
		t.Errorf("zones = %+v, want one deduplicated card", ctx.Zones)
	}
	for _, r := range ctx.Rules {
		if len(r.Via) == 0 {
			t.Errorf("rule %s carries no via in a multi-part scope", r.Summary.ID)
		}
	}
	if ctx.Scope != "m/service.go, m/other.go, zone m" {
		t.Errorf("scope = %q", ctx.Scope)
	}

	if _, err := uc.Execute(application.ContextRequest{Zones: []string{"ghost"}}); err == nil ||
		!strings.Contains(err.Error(), "not declared") {
		t.Errorf("unknown zone err = %v", err)
	}
}

func TestArchitecturalContextRepositoryTeaches(t *testing.T) {
	uc, err := application.NewGetArchitecturalContext(fakeRepository{contextFixture(t)}, emptyKnowledge())
	if err != nil {
		t.Fatalf("NewGetArchitecturalContext: %v", err)
	}
	ctx, err := uc.Execute(application.ContextRequest{})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if ctx.UnknownImports != "warn" {
		t.Errorf("unknown-imports posture = %q, want the warn default", ctx.UnknownImports)
	}
	kinds := map[string]string{}
	for _, k := range ctx.Kinds {
		kinds[k.Kind] = k.Meaning
	}
	for _, want := range []string{"naming", "consumes", "protected"} {
		if kinds[want] == "" {
			t.Errorf("kind %q missing or meaningless in %v", want, kinds)
		}
	}
	if len(ctx.Paths) != 0 || len(ctx.Rules) != 0 {
		t.Errorf("repository scope must carry no bindings or rule union: %+v", ctx)
	}
}
