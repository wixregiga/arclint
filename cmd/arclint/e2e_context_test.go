package main

// The context command is the worksite call: one invocation answers
// what governs a set of paths and modules, and the bare form explains
// the repository. The JSON shape asserted here is the agent contract.

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestContextWorksite(t *testing.T) {
	stdout, stderr, code := runBin(t, repoRoot(t), os.Environ(), "context",
		"internal/domain/rule/root.go", "internal/infrastructure/rule/yaml/yaml.go", "--format", "json")
	if code != 0 {
		t.Fatalf("context worksite: exit %d\nstderr: %s", code, stderr)
	}
	var ctx struct {
		Scope string
		Paths []struct {
			Path    string
			Modules []string
		}
		Modules []struct{ Name string }
		Rules   []struct {
			Summary struct{ ID string }
			Via     []string
		}
	}
	if err := json.Unmarshal([]byte(stdout), &ctx); err != nil {
		t.Fatalf("context json: %v\n%s", err, stdout)
	}
	if len(ctx.Paths) != 2 {
		t.Fatalf("path bindings = %+v", ctx.Paths)
	}
	bound := map[string]bool{}
	for _, b := range ctx.Paths {
		for _, m := range b.Modules {
			bound[b.Path+"→"+m] = true
		}
	}
	if !bound["internal/domain/rule/root.go→domain"] || !bound["internal/infrastructure/rule/yaml/yaml.go→infrastructure"] {
		t.Errorf("bindings missing expected modules: %+v", ctx.Paths)
	}
	cards := map[string]bool{}
	for _, m := range ctx.Modules {
		if cards[m.Name] {
			t.Errorf("module card %q duplicated", m.Name)
		}
		cards[m.Name] = true
	}
	if !cards["domain"] || !cards["infrastructure"] {
		t.Errorf("module cards = %v", cards)
	}
	rules := map[string][]string{}
	for _, r := range ctx.Rules {
		rules[r.Summary.ID] = r.Via
	}
	// The boundary rule protecting infrastructure joins the union, and
	// the extension rule binds through the domain path.
	if _, ok := rules["arclint:infrastructure/composition-only"]; !ok {
		t.Errorf("boundary rule missing from union: %v", rules)
	}
	if via := rules["arclint:domain/no-panic"]; len(via) == 0 || via[0] != "internal/domain/rule/root.go" {
		t.Errorf("extension rule via = %v", via)
	}
}

func TestContextModuleScopeAndErrors(t *testing.T) {
	stdout, _, code := runBin(t, repoRoot(t), os.Environ(), "context", "--module", "domain", "--format", "json")
	if code != 0 {
		t.Fatalf("context --module: exit %d", code)
	}
	var ctx struct {
		Scope string
		Rules []struct {
			Summary struct{ ID string }
		}
	}
	if err := json.Unmarshal([]byte(stdout), &ctx); err != nil {
		t.Fatalf("json: %v", err)
	}
	if ctx.Scope != "module domain" {
		t.Errorf("scope = %q", ctx.Scope)
	}
	ids := map[string]bool{}
	for _, r := range ctx.Rules {
		ids[r.Summary.ID] = true
	}
	if !ids["arclint:domain/stdlib-only"] {
		t.Errorf("module scope misses the module's own consumes rule: %v", ids)
	}

	_, stderr, code := runBin(t, repoRoot(t), os.Environ(), "context", "--module", "ghost")
	if code != 2 || !strings.Contains(stderr, "not declared") {
		t.Errorf("unknown module: exit %d, stderr %s", code, stderr)
	}
}

func TestContextRepositoryTeaches(t *testing.T) {
	stdout, _, code := runBin(t, repoRoot(t), os.Environ(), "context")
	if code != 0 {
		t.Fatalf("bare context: exit %d", code)
	}
	for _, want := range []string{
		"rule types in use:",
		"protected — restricts which Modules may import one Module",
		"unknown imports: error",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("bare context missing %q:\n%.400s", want, stdout)
		}
	}
}

func TestCompletionModuleNames(t *testing.T) {
	stdout, _, code := runBin(t, repoRoot(t), os.Environ(), "__complete", "context", "--module", "")
	if code != 0 || !strings.Contains(stdout, "domain\t") {
		t.Errorf("--module completion: exit %d\n%s", code, stdout)
	}
	stdout, _, code = runBin(t, repoRoot(t), os.Environ(), "__complete", "context", "--module", "domain,")
	if code != 0 || !strings.Contains(stdout, "domain,application\t") {
		t.Errorf("--module comma completion: exit %d\n%s", code, stdout)
	}
}
