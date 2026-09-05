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
	// the content rule binds through the domain path.
	if _, ok := rules["infrastructure/composition-only"]; !ok {
		t.Errorf("boundary rule missing from union: %v", rules)
	}
	if via := rules["domain/no-panic"]; len(via) == 0 || via[0] != "internal/domain/rule/root.go" {
		t.Errorf("content rule via = %v", via)
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
	if !ids["domain/stdlib-only"] {
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
		"protected: restricts which Modules may import one Module",
		"unknown imports: error",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("bare context missing %q:\n%.400s", want, stdout)
		}
	}
}

// TestContextWorksiteScopesDomain pins the progressive disclosure of
// the recorded domain: a worksite lists only what anchors into it,
// says so in the headline, and reports the unanchored contracts
// loudly; --full restores the whole model.
func TestContextWorksiteScopesDomain(t *testing.T) {
	stdout, stderr, code := runBin(t, repoRoot(t), os.Environ(), "context", "internal/domain/rule/root.go")
	if code != 0 {
		t.Fatalf("context worksite: exit %d\nstderr: %s", code, stderr)
	}
	for _, want := range []string{
		"project domain (domain.arclint.yaml): 1 of 4 contexts,",
		"anchor into this scope; --full shows the whole model\n",
		"  context rule:\n",
		"    entities: Rule [aggregate]\n",
		"  unanchored contracts: 3 unanchorable\n",
		"    unanchorable: 3 invariants owned by Rule (context rule)\n",
		"      owner Rule is an aggregate and the invariant has no id, so no method is named to carry it\n",
		"    an unanchorable contract needs its recording changed before any source can carry it\n",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("scoped context missing %q:\n%s", want, stdout)
		}
	}
	for _, absent := range []string{"context adoption:", "context conformance:", "context distribution:"} {
		if strings.Contains(stdout, absent) {
			t.Errorf("scoped context leaks %q:\n%s", absent, stdout)
		}
	}

	full, stderr, code := runBin(t, repoRoot(t), os.Environ(), "context", "internal/domain/rule/root.go", "--full")
	if code != 0 {
		t.Fatalf("context --full: exit %d\nstderr: %s", code, stderr)
	}
	if strings.Contains(full, "--full") {
		t.Errorf("a full listing must not point at --full:\n%s", full)
	}
	for _, want := range []string{"context rule:", "context adoption:", "context conformance:", "context distribution:", "unanchored contracts: 5 unanchorable, 8 missing"} {
		if !strings.Contains(full, want) {
			t.Errorf("full context missing %q:\n%s", want, full)
		}
	}

	stdout, stderr, code = runBin(t, repoRoot(t), os.Environ(), "context", "internal/domain/rule/root.go", "--format", "json")
	if code != 0 {
		t.Fatalf("context worksite json: exit %d\nstderr: %s", code, stderr)
	}
	var ctx struct {
		Domain struct {
			Scoped   bool
			Located  bool
			Counts   struct{ Contexts, Invariants int }
			Shown    struct{ Contexts, Invariants int }
			Contexts []struct {
				Name       string
				Invariants []struct {
					Owner  string
					Anchor string
					Reason string
				}
			}
			Unanchored []struct {
				Kind, Context, Owner, Anchor, Reason string
			}
		}
	}
	if err := json.Unmarshal([]byte(stdout), &ctx); err != nil {
		t.Fatalf("context json: %v\n%s", err, stdout)
	}
	d := ctx.Domain
	if !d.Scoped || !d.Located {
		t.Fatalf("scoped=%v located=%v", d.Scoped, d.Located)
	}
	if d.Counts.Contexts != 4 || d.Shown.Contexts != 1 || d.Shown.Invariants != 3 {
		t.Fatalf("counts %+v shown %+v", d.Counts, d.Shown)
	}
	if len(d.Contexts) != 1 || d.Contexts[0].Name != "rule" {
		t.Fatalf("contexts = %+v", d.Contexts)
	}
	for _, inv := range d.Contexts[0].Invariants {
		if inv.Owner != "Rule" || inv.Anchor != "unanchorable" || inv.Reason == "" {
			t.Errorf("invariant = %+v", inv)
		}
	}
	if len(d.Unanchored) != 3 {
		t.Fatalf("unanchored = %+v", d.Unanchored)
	}
	for _, u := range d.Unanchored {
		if u.Kind != "invariant" || u.Context != "rule" || u.Owner != "Rule" || u.Anchor != "unanchorable" || u.Reason == "" {
			t.Errorf("unanchored entry = %+v", u)
		}
	}
}

// TestContextOutsideDomainSaysSo pins the empty-scope headline: a path
// nothing recorded anchors into still names the whole model and the
// flag that shows it.
func TestContextOutsideDomainSaysSo(t *testing.T) {
	stdout, stderr, code := runBin(t, repoRoot(t), os.Environ(), "context", "cmd/arclint/main.go")
	if code != 0 {
		t.Fatalf("context main.go: exit %d\nstderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "project domain (domain.arclint.yaml): nothing recorded anchors into this scope; --full shows the whole model (4 contexts,") {
		t.Fatalf("empty-scope headline missing:\n%s", stdout)
	}
	if strings.Contains(stdout, "  context ") || strings.Contains(stdout, "unanchored contracts:") {
		t.Fatalf("empty scope must list nothing:\n%s", stdout)
	}
}

// TestContextModuleNamedForContextKeepsItWhole pins the Module route
// into the domain: a Module whose name matches a recorded context
// keeps that context whole.
func TestContextModuleNamedForContextKeepsItWhole(t *testing.T) {
	stdout, stderr, code := runBin(t, repoRoot(t), os.Environ(), "context", "--module", "rule", "--format", "json")
	if code != 0 {
		t.Fatalf("context --module rule: exit %d\nstderr: %s", code, stderr)
	}
	var ctx struct {
		Domain struct {
			Scoped   bool
			Contexts []struct {
				Name         string
				Entities     []struct{ Name string }
				ValueObjects []string
			}
		}
	}
	if err := json.Unmarshal([]byte(stdout), &ctx); err != nil {
		t.Fatalf("context json: %v\n%s", err, stdout)
	}
	if !ctx.Domain.Scoped {
		t.Fatalf("module scope must be scoped: %+v", ctx.Domain)
	}
	var ruleCtx *struct {
		Name         string
		Entities     []struct{ Name string }
		ValueObjects []string
	}
	for i := range ctx.Domain.Contexts {
		if ctx.Domain.Contexts[i].Name == "rule" {
			ruleCtx = &ctx.Domain.Contexts[i]
		}
	}
	if ruleCtx == nil {
		t.Fatalf("context rule missing: %+v", ctx.Domain.Contexts)
	}
	if len(ruleCtx.Entities) != 3 || len(ruleCtx.ValueObjects) < 10 {
		t.Fatalf("context rule is not whole: %+v", *ruleCtx)
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
