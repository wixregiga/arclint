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
	scope, err := rule.ModuleApplicability([]rule.ModuleName{"m"})
	if err != nil {
		t.Fatalf("ModuleApplicability: %v", err)
	}
	consumes, err := rule.New(rule.Spec{
		ID:            "t:m/imports",
		Type:          rule.TypeConsumes,
		Params:        rule.ConsumesParams{Internal: &emptyAllow, External: rule.ImportForbid},
		Applicability: scope,
	})
	if err != nil {
		t.Fatalf("rule.New: %v", err)
	}
	repo, err := rule.RepositoryApplicability()
	if err != nil {
		t.Fatalf("RepositoryApplicability: %v", err)
	}
	protected, err := rule.New(rule.Spec{
		ID:            "t:deps/protected-m",
		Type:          rule.TypeProtected,
		Params:        rule.ProtectedParams{Module: "m"},
		Applicability: repo,
	})
	if err != nil {
		t.Fatalf("rule.New: %v", err)
	}
	cfg.Rules = append(cfg.Rules, consumes, protected)
	return cfg
}

func TestArchitecturalContextForPath(t *testing.T) {
	uc, err := application.NewGetArchitecturalContext(fakeRepository{contextFixture(t)})
	if err != nil {
		t.Fatalf("NewGetArchitecturalContext: %v", err)
	}
	ctx, err := uc.Execute("m/service.go")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(ctx.Modules) != 1 || ctx.Modules[0].Name != "m" {
		t.Fatalf("owning modules = %+v, want m", ctx.Modules)
	}
	policy := ctx.Modules[0]
	if !policy.InternalRestricted || len(policy.Internal) != 0 || policy.External != "forbid" {
		t.Errorf("module policy = %+v, want restricted-empty internal and forbidden external", policy)
	}
	reasons := map[string]string{}
	for _, r := range ctx.Rules {
		reasons[r.Summary.ID] = r.Reason
	}
	if !strings.Contains(reasons["t:m/snake"], "Module(s) m") {
		t.Errorf("naming reason = %q", reasons["t:m/snake"])
	}
	if !strings.Contains(reasons["t:deps/protected-m"], "protected") {
		t.Errorf("protected reason = %q", reasons["t:deps/protected-m"])
	}
	if _, ok := reasons["t:m/imports"]; !ok {
		t.Errorf("consumes rule missing from applicable rules: %v", reasons)
	}

	outside, err := uc.Execute("elsewhere/file.go")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(outside.Modules) != 0 {
		t.Errorf("a path outside every module must own no modules, got %+v", outside.Modules)
	}
}

func TestPublishAgentsContextRendersAndInstalls(t *testing.T) {
	getContext, err := application.NewGetArchitecturalContext(fakeRepository{contextFixture(t)})
	if err != nil {
		t.Fatalf("NewGetArchitecturalContext: %v", err)
	}
	publisher := &fakePublisher{}
	publish, err := application.NewPublishAgentsContext(getContext, publisher)
	if err != nil {
		t.Fatalf("NewPublishAgentsContext: %v", err)
	}
	block, err := publish.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	for _, want := range []string{
		application.AgentsBegin, application.AgentsEnd,
		"3 rules over languages [go]", "- **m**",
		"internal imports none (may import no other declared module)",
		"arclint context <path>",
	} {
		if !strings.Contains(block, want) {
			t.Errorf("block lacks %q:\n%s", want, block)
		}
	}
	if _, _, err := publish.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if publisher.installed != block {
		t.Errorf("installed block differs from rendered block")
	}
}

type fakePublisher struct {
	installed string
}

func (f *fakePublisher) Install(block string) (bool, string, error) {
	f.installed = block
	return true, "AGENTS.md", nil
}

func TestInitializeRepositoryRejectsUnknownLanguages(t *testing.T) {
	uc, err := application.NewInitializeRepository(fakeScaffold{})
	if err != nil {
		t.Fatalf("NewInitializeRepository: %v", err)
	}
	if _, err := uc.Execute(application.InitializeRepositoryRequest{Languages: []string{"rust"}}); err == nil {
		t.Errorf("unsupported language must be rejected")
	}
}

type fakeScaffold struct{}

func (fakeScaffold) Write(content string, force bool) (string, error) { return "rules.yaml", nil }
