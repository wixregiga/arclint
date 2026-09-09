package yamlrule_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
	embeddedpattern "github.com/wixregiga/arclint/internal/infrastructure/pattern/embedded"
	yamlrule "github.com/wixregiga/arclint/internal/infrastructure/rule/yaml"
	yamlvocab "github.com/wixregiga/arclint/internal/infrastructure/vocab/yaml"
)

// repoVocabulary is this repository's recorded Ubiquitous Language:
// the loader resolves expanded Rules against it, so the target-ruleset
// test must load it exactly as production composition does.
func repoVocabulary(t *testing.T) vocab.Repository {
	t.Helper()
	repo, err := yamlvocab.NewRepository("../../../..")
	if err != nil {
		t.Fatalf("vocabulary repository: %v", err)
	}
	return repo
}

// TestLoadTargetRuleset proves the loader against the real target
// ruleset of this repository: the local Rules it spells, plus the
// built-in vocabulary rules every ruleset carries without extending
// any Pattern, exactly as an adopter receives them.
func TestLoadTargetRuleset(t *testing.T) {
	repo, err := yamlrule.NewRepository("../../../../rules.arclint.yaml", repoVocabulary(t), embeddedpattern.NewSource())
	if err != nil {
		t.Fatalf("NewRepository: %v", err)
	}
	cfg, err := repo.ConfiguredRules()
	if err != nil {
		t.Fatalf("ConfiguredRules: %v", err)
	}
	if len(cfg.Zones) != 17 {
		t.Errorf("zones = %d, want 17", len(cfg.Zones))
	}
	builtIn, err := rule.BuiltIn()
	if err != nil {
		t.Fatalf("BuiltIn: %v", err)
	}
	const local = 29
	if len(cfg.Rules) != local+len(builtIn) {
		t.Errorf("rules = %d, want %d local + %d built-in", len(cfg.Rules), local, len(builtIn))
	}
	if len(cfg.Languages) != 2 || cfg.Languages[0] != rule.LanguageGo || cfg.Languages[1] != rule.LanguageTypeScript {
		t.Errorf("languages = %v, want [go typescript]", cfg.Languages)
	}
	if cfg.Scan.UnknownImports != rule.UnknownImportsError {
		t.Errorf("unknown imports policy = %q, want error", cfg.Scan.UnknownImports)
	}
	if len(cfg.Extensions) != 0 {
		t.Errorf("the repository extends no Pattern and authors no extension; got %d extensions", len(cfg.Extensions))
	}
	byID := map[string]rule.Rule{}
	for _, r := range cfg.Rules {
		byID[r.ID().Qualified()] = r
		if r.Proposition() == "" {
			t.Errorf("%s: every Rule states its Constraint", r.ID().Qualified())
		}
		if !r.BuiltIn() && r.Rationale().IsZero() {
			t.Errorf("%s: the authored rationale was lost", r.ID().Qualified())
		}
		if _, distributed := r.Provenance(); distributed {
			t.Errorf("%s: no Rule of this ruleset comes from a Pattern", r.ID().Qualified())
		}
	}
	// The vocabulary rules are built in: listed under the invariant's
	// id, carrying no provenance, and never spelled in the local file.
	for _, id := range []string{
		"ubiquitous_language/terms-declared-in-code",
		"context_relation/imports-follow-influence",
		"aggregate/root-declared",
		"domain_isolation/model-imports-nothing-outside-itself",
	} {
		r, ok := byID[id]
		if !ok {
			t.Errorf("missing built-in %s; have %v", id, keys(byID))
			continue
		}
		if !r.BuiltIn() {
			t.Errorf("%s must be marked built in", id)
		}
	}
	for _, retired := range []string{
		"arclint/domain-model:vocabulary/terms-carry-definitions",
		"arclint/domain-model:contexts/respect-relations",
		"vocabulary/terms-carry-definitions",
	} {
		if _, ok := byID[retired]; ok {
			t.Errorf("%s is retired; the built-in rules replace the domain-model Pattern", retired)
		}
	}
	stdlibOnly, ok := byID["domain/stdlib-only"]
	if !ok {
		t.Fatalf("missing domain/stdlib-only; have %v", keys(byID))
	}
	if stdlibOnly.Type() != rule.TypeConsumes {
		t.Errorf("stdlib-only type = %q", stdlibOnly.Type())
	}
	if !strings.Contains(stdlibOnly.Rationale().String(), "no other Zone") {
		t.Errorf("stdlib-only rationale = %q", stdlibOnly.Rationale())
	}
	if acyclic, ok := byID["dependencies/acyclic"]; !ok {
		t.Errorf("missing dependencies/acyclic")
	} else if acyclic.Type() != rule.TypeAcyclic {
		t.Errorf("acyclic type = %q", acyclic.Type())
	}
	if table, ok := byID["infrastructure/stdlib-table-present"]; !ok {
		t.Errorf("missing infrastructure/stdlib-table-present")
	} else if table.Type() != rule.TypeStructure {
		t.Errorf("stdlib-table rule type = %q, want structure", table.Type())
	}
	if noPanic, ok := byID["domain/no-panic"]; !ok {
		t.Errorf("missing domain/no-panic")
	} else if p, ok := noPanic.Params().(rule.ContentParams); !ok || p.Forbid != `\bpanic\(` {
		t.Errorf("no-panic params = %#v, want the content forbid", noPanic.Params())
	}
	skeleton, ok := byID["domain-model/aggregate-skeleton"]
	if !ok {
		t.Fatalf("missing domain-model/aggregate-skeleton")
	}
	if _, expanded := skeleton.Expansion(); !expanded {
		t.Errorf("aggregate-skeleton carries no expansion")
	}
	params, ok := skeleton.Params().(rule.StructureParams)
	if !ok || len(params.Require) != 2 {
		// The repository records one aggregate (Rule), so the two
		// authored globs resolve to exactly two obligations.
		t.Errorf("aggregate-skeleton params = %#v", skeleton.Params())
	}
}

func keys(m map[string]rule.Rule) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func loadString(t *testing.T, content string, available ...rule.Pattern) (yamlrule.Document, error) {
	t.Helper()
	return yamlrule.Load([]byte(content), "test.yaml", vocab.UbiquitousLanguage{}, available)
}

func mustLoad(t *testing.T, content string, available ...rule.Pattern) rule.Configured {
	t.Helper()
	doc, err := loadString(t, content, available...)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return doc.Configured
}

func ruleByID(t *testing.T, cfg rule.Configured, id string) rule.Rule {
	t.Helper()
	for _, r := range cfg.Rules {
		if r.ID().Qualified() == id {
			return r
		}
	}
	t.Fatalf("missing rule %s; have %v", id, ruleIDs(cfg))
	return rule.Rule{}
}

func ruleIDs(cfg rule.Configured) []string {
	out := make([]string, 0, len(cfg.Rules))
	for _, r := range cfg.Rules {
		out = append(out, r.ID().Qualified())
	}
	return out
}

// TestLoadEveryConstraintShape proves each constraint key, each zone
// sugar, and each optional key loads into the Rule Type it spells.
func TestLoadEveryConstraintShape(t *testing.T) {
	cfg := mustLoad(t, `
runtime: [go, ts]
scan:
  unknown_imports: ignore
  exclude: ["vendor/**"]
  include_testdata: true
zones:
  domain: internal/domain/**
  application: ["internal/application/**", "internal/usecases/**"]
  infra:
    paths: internal/infra/**
    description: "Outbound adapters."
  app:
    paths: ["cmd/**"]
rules:
  domain/stdlib-only:
    rationale: "The domain imports nothing else."
    on: domain
    imports:
      internal: []
      external: forbid
  application/imports-domain:
    on: [application, infra]
    imports:
      internal: [domain]
  domain/root-present:
    on: domain
    structure:
      require: ["internal/domain/root.go"]
      forbid: ["internal/domain/util.go"]
  domain/each-aggregate:
    severity: warning
    on: domain
    structure:
      each: domain.aggregates
      require: ["internal/domain/{name:flatcase}/root.go"]
  domain/snake:
    on: domain
    files: "internal/domain/**/*.go"
    naming: snake_case
  application/kebab:
    on: application
    naming:
      case: "kebab-case|regex:^v[0-9]+$"
  domain/no-panic:
    on: domain
    files: ["internal/domain/**/*.go"]
    content:
      forbid: '\bpanic\('
  repo/no-todo:
    severity: info
    content:
      forbid: "TODO"
  deps/inward:
    layers: [app, application, domain]
  infra/app-only:
    on: infra
    imported_by: [app]
  features/independent:
    independent: ["internal/features/*"]
  deps/acyclic-all:
    acyclic: {}
  deps/acyclic-some:
    acyclic: [app, domain]
  domain/checked:
    on: domain
    uses: acme/check
    with:
      depth: 2
  repo/checked:
    uses: acme/repo-check
    exclude:
      paths: ["internal/generated/**"]
      reason: "generated code is not authored"
    suppress:
      paths: ["internal/legacy/**"]
      reason: "adopted debt tracked in the baseline"
`)
	if len(cfg.Languages) != 2 || cfg.Languages[1] != rule.LanguageTypeScript {
		t.Errorf("languages = %v", cfg.Languages)
	}
	if cfg.Scan.UnknownImports != rule.UnknownImportsIgnore || !cfg.Scan.IncludeTestdata || len(cfg.Scan.Exclude) != 1 {
		t.Errorf("scan = %+v", cfg.Scan)
	}
	if len(cfg.Zones) != 4 {
		t.Fatalf("zones = %d, want 4", len(cfg.Zones))
	}
	if paths := cfg.Zones[0].Paths(); len(paths) != 1 || paths[0].String() != "internal/domain/**" {
		t.Errorf("string zone paths = %v", paths)
	}
	if paths := cfg.Zones[1].Paths(); len(paths) != 2 {
		t.Errorf("list zone paths = %v", paths)
	}
	if cfg.Zones[2].Description() != "Outbound adapters." {
		t.Errorf("object zone description = %q", cfg.Zones[2].Description())
	}
	want := map[string]rule.Type{
		"domain/stdlib-only":         rule.TypeConsumes,
		"application/imports-domain": rule.TypeConsumes,
		"domain/root-present":        rule.TypeStructure,
		"domain/each-aggregate":      rule.TypeStructure,
		"domain/snake":               rule.TypeNaming,
		"application/kebab":          rule.TypeNaming,
		"domain/no-panic":            rule.TypeContent,
		"repo/no-todo":               rule.TypeContent,
		"deps/inward":                rule.TypeLayers,
		"infra/app-only":             rule.TypeProtected,
		"features/independent":       rule.TypeIndependence,
		"deps/acyclic-all":           rule.TypeAcyclic,
		"deps/acyclic-some":          rule.TypeAcyclic,
		"domain/checked":             rule.TypeExtension,
		"repo/checked":               rule.TypeExtension,
	}
	if len(cfg.Rules) != len(want) {
		t.Fatalf("rules = %v, want %d", ruleIDs(cfg), len(want))
	}
	for id, typ := range want {
		if got := ruleByID(t, cfg, id).Type(); got != typ {
			t.Errorf("%s type = %s, want %s", id, got, typ)
		}
	}
	if r := ruleByID(t, cfg, "domain/stdlib-only"); r.Rationale().String() != "The domain imports nothing else." {
		t.Errorf("rationale must preserve the authored reason, got %q", r.Rationale())
	}
	if p := ruleByID(t, cfg, "application/imports-domain").Params().(rule.ConsumesParams); p.Internal == nil || len(p.Internal.Zones()) != 1 || p.External != rule.ImportAllow {
		t.Errorf("imports params = %+v", p)
	}
	if r := ruleByID(t, cfg, "domain/each-aggregate"); r.Severity() != rule.SeverityWarning {
		t.Errorf("severity = %s", r.Severity())
	} else if _, expanded := r.Expansion(); !expanded {
		t.Errorf("each must record an expansion")
	} else if p := r.Params().(rule.StructureParams); len(p.Require) != 0 {
		t.Errorf("an empty vocabulary derives no obligations, got %v", p.Require)
	}
	if r := ruleByID(t, cfg, "domain/snake"); !r.AppliesToFile("internal/domain/a.go", []rule.ZoneName{"domain"}) ||
		r.AppliesToFile("internal/domain/README.md", []rule.ZoneName{"domain"}) {
		t.Errorf("files must narrow the naming Rule")
	}
	if p := ruleByID(t, cfg, "repo/no-todo").Params().(rule.ContentParams); p.Forbid != "TODO" {
		t.Errorf("content params = %+v", p)
	}
	if r := ruleByID(t, cfg, "repo/no-todo"); len(r.ReferencedZones()) != 0 || r.Severity() != rule.SeverityInfo {
		t.Errorf("a content Rule without on ranges over the repository; got %v %s", r.ReferencedZones(), r.Severity())
	}
	if p := ruleByID(t, cfg, "infra/app-only").Params().(rule.ProtectedParams); p.Zone != "infra" || len(p.Allow) != 1 || p.Allow[0] != "app" {
		t.Errorf("imported_by params = %+v", p)
	}
	if p := ruleByID(t, cfg, "deps/acyclic-all").Params().(rule.AcyclicParams); len(p.Zones) != 0 {
		t.Errorf("acyclic {} means every declared zone, got %v", p.Zones)
	}
	if p := ruleByID(t, cfg, "deps/acyclic-some").Params().(rule.AcyclicParams); len(p.Zones) != 2 {
		t.Errorf("acyclic list = %v", p.Zones)
	}
	if p := ruleByID(t, cfg, "domain/checked").Params().(rule.ExtensionParams); p.Uses != "acme/check" || p.With["depth"] != 2 {
		t.Errorf("uses params = %+v", p)
	}
	checked := ruleByID(t, cfg, "repo/checked")
	if checked.AppliesToFile("internal/generated/x.go", nil) {
		t.Errorf("exclude must remove the path from what the Rule judges")
	}
	if _, ok := checked.SuppressionFor("internal/legacy/x.go"); !ok {
		t.Errorf("suppress must keep the path but remove its gate effect")
	}
}

// TestLoadRejectsInvalidDocuments pins the strict grammar: every
// malformed shape fails loudly with a message naming the fault.
func TestLoadRejectsInvalidDocuments(t *testing.T) {
	cases := map[string]struct {
		document string
		want     string
	}{
		"empty document":        {"", "empty document"},
		"unknown top-level key": {"rulesets: []\n", `unknown key "rulesets"`},
		"retired contracts key": {`
zones:
  m: m/**
contracts:
  m:
    consumes:
      id: "repo/p:m/imports"
      internal: []
`, `unknown key "contracts"`},
		"rule without constraint and without extends": {`
zones:
  m: m/**
rules:
  m/imports:
    rationale: "nothing"
    on: m
`, "carries no constraint"},
		"two constraints": {`
zones:
  m: m/**
rules:
  m/two:
    on: m
    imports:
      internal: []
    naming: snake_case
`, "carries 2 constraints"},
		"kind key": {`
zones:
  m: m/**
rules:
  m/kind:
    kind: naming
    on: m
    case: snake_case
`, `unknown key "kind"`},
		"imports without on": {`
zones:
  m: m/**
rules:
  m/imports:
    imports:
      internal: []
`, "constraint does not accept its scope"},
		"on for layers": {`
zones:
  m: m/**
  n: n/**
rules:
  deps/inward:
    on: m
    layers: [m, n]
`, "constraint does not accept its scope"},
		"imported_by with two zones": {`
zones:
  m: m/**
  n: n/**
rules:
  m/protected:
    on: [m, n]
    imported_by: [n]
`, "imported_by requires on naming exactly one zone"},
		"on names an undeclared zone": {`
zones:
  m: m/**
rules:
  ghost/imports:
    on: ghost
    imports:
      internal: []
`, `zone "ghost" is not declared`},
		"allow-list names an undeclared zone": {`
zones:
  m: m/**
rules:
  m/imports:
    on: m
    imports:
      internal: [ghost]
`, `names zone "ghost", which is not declared`},
		"files on imports": {`
zones:
  m: m/**
rules:
  m/imports:
    on: m
    files: "m/**/*.go"
    imports:
      internal: []
`, "imports does not accept files"},
		"with on imports": {`
zones:
  m: m/**
rules:
  m/imports:
    on: m
    imports:
      internal: []
    with:
      depth: 1
`, "with belongs to uses"},
		"content without forbid": {`
zones:
  m: m/**
rules:
  m/content:
    on: m
    content: {}
`, "forbid is required"},
		"content with invalid regexp": {`
zones:
  m: m/**
rules:
  m/content:
    on: m
    content:
      forbid: "("
`, "forbid"},
		"naming with unknown case": {`
zones:
  m: m/**
rules:
  m/naming:
    on: m
    naming: SCREAMING_CASE
`, "SCREAMING_CASE"},
		"acyclic with one zone": {`
zones:
  m: m/**
rules:
  deps/acyclic:
    acyclic: [m]
`, "at least two zones"},
		"duplicate rule id": {`
zones:
  m: m/**
rules:
  m/same:
    on: m
    imports:
      internal: []
  m/same:
    on: m
    naming: snake_case
`, `key "m/same" appears twice`},
		"disable without reason": {`
zones:
  m: m/**
rules:
  m/imports:
    on: m
    imports:
      internal: []
    disable: ""
`, "a reason is required"},
		"exclusion without reason": {`
zones:
  m: m/**
rules:
  m/imports:
    on: m
    imports:
      internal: []
    exclude:
      paths: ["m/gen/**"]
`, "reason is required"},
		"exclusion without subject": {`
zones:
  m: m/**
rules:
  m/imports:
    on: m
    imports:
      internal: []
    exclude:
      reason: "why"
`, "names no paths and no zones"},
		"suppression without paths": {`
zones:
  m: m/**
rules:
  m/imports:
    on: m
    imports:
      internal: []
    suppress:
      reason: "why"
`, "paths is required"},
		"repository zone without paths": {`
zones:
  m:
    description: "only a description"
rules: {}
`, "a repository zone requires paths"},
		"zone with unknown key": {`
zones:
  m:
    paths: m/**
    globs: ["m/**"]
`, `unknown key "globs"`},
		"runtime unknown target":   {"runtime: [rust]\n", `"rust": not one of go, ts, py`},
		"runtime duplicate target": {"runtime: [go, go]\n", "listed twice"},
		"runtime empty":            {"runtime: []\n", "names no language"},
		"scan unknown policy":      {"scan:\n  unknown_imports: explode\n", "scan"},
		"extends unknown pattern": {`
extends:
  - pattern: acme/hexagonal@1.0.0
`, "pattern acme/hexagonal@1.0.0 is not available; known patterns: none"},
		"extends without pattern": {`
extends:
  - bind:
      m: m/**
`, "pattern is required"},
		"extends inexact version": {`
extends:
  - pattern: acme/hexagonal@latest
`, "not exact semver"},
		"pattern file with runtime": {`
pattern:
  namespace: acme
  name: hexagonal
  version: 1.0.0
runtime: [go]
`, "does not accept runtime"},
		"pattern file with scan": {`
pattern:
  namespace: acme
  name: hexagonal
  version: 1.0.0
scan:
  unknown_imports: warn
`, "does not accept scan"},
		"pattern file with extends": {`
pattern:
  namespace: acme
  name: hexagonal
  version: 1.0.0
extends: []
`, "does not accept extends"},
		"pattern file with list zone": {`
pattern:
  namespace: acme
  name: hexagonal
  version: 1.0.0
zones:
  core: ["core/**"]
`, "a pattern lists a zone by its description"},
		"pattern header missing version": {`
pattern:
  namespace: acme
  name: hexagonal
`, "version is required"},
		"pattern header unknown key": {`
pattern:
  namespace: acme
  name: hexagonal
  version: 1.0.0
  author: me
`, `unknown key "author"`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := loadString(t, tc.document)
			if err == nil {
				t.Fatalf("expected a load error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to mention %q", err, tc.want)
			}
		})
	}
}

// samplePatternFile is a Pattern distribution file in the published
// grammar: header, Zones by description, Rules under local ids.
const samplePatternFile = `
pattern:
  namespace: acme
  name: hexagonal
  version: 1.0.0
  coverage: [go, ts]
  documentation: https://example.test/hexagonal
zones:
  core: "The domain core."
  ports:
    description: "Inbound and outbound ports."
    paths: ["internal/ports/**"]
  adapters:
    description: "Technology adapters."
rules:
  core/stdlib-only:
    rationale: "The core imports no other Zone and no third-party package."
    on: core
    imports:
      internal: []
      external: forbid
  core/no-panic:
    rationale: "The core never panics."
    on: core
    files: "**/*.go"
    content:
      forbid: '\bpanic\('
  core/aggregates:
    rationale: "Every recorded aggregate has a root in the core."
    severity: warning
    on: core
    structure:
      each: domain.aggregates
      require: ["{name:flatcase}/root.go"]
  adapters/core-only:
    on: adapters
    imported_by: [ports]
  deps/acyclic:
    acyclic: {}
  core/checked:
    on: core
    uses: acme/check
`

func loadSamplePattern(t *testing.T) rule.Pattern {
	t.Helper()
	ext, err := rule.NewPatternExtension("acme_check.ts", "export default { type: \"acme/check\" }")
	if err != nil {
		t.Fatalf("NewPatternExtension: %v", err)
	}
	p, err := yamlrule.LoadPattern([]byte(samplePatternFile), "acme/hexagonal/pattern.yaml", []rule.PatternExtension{ext})
	if err != nil {
		t.Fatalf("LoadPattern: %v", err)
	}
	return p
}

// TestLoadPatternQualifiesLocalIDs proves a Pattern file's local Rule
// IDs are qualified with the Pattern namespace, its Zones carry
// descriptions and suggested paths, and its Rules carry provenance.
func TestLoadPatternQualifiesLocalIDs(t *testing.T) {
	p := loadSamplePattern(t)
	if p.Reference().String() != "acme/hexagonal@1.0.0" || p.Documentation() != "https://example.test/hexagonal" {
		t.Errorf("pattern = %s %q", p.Reference(), p.Documentation())
	}
	// coverage is spelled exactly like runtime and resolves to the same
	// Languages.
	if cov := p.Coverage(); len(cov) != 2 || cov[0] != rule.LanguageGo || cov[1] != rule.LanguageTypeScript {
		t.Errorf("coverage = %v, want [go typescript]", cov)
	}
	if len(p.Rules()) != 6 {
		t.Fatalf("rules = %d, want 6", len(p.Rules()))
	}
	for _, r := range p.Rules() {
		if r.ID().Namespace() != "acme" {
			t.Errorf("%s must be qualified with the pattern namespace", r.ID().Qualified())
		}
		if ref, ok := r.Provenance(); !ok || ref.Name() != "hexagonal" {
			t.Errorf("%s provenance = %v %v", r.ID().Qualified(), ref, ok)
		}
	}
	if p.Rules()[0].ID().Qualified() != "acme/hexagonal:core/stdlib-only" {
		t.Errorf("first rule = %s", p.Rules()[0].ID().Qualified())
	}
	zones := p.Zones()
	if len(zones) != 3 || zones[0].Description() != "The domain core." || len(zones[0].SuggestedPaths()) != 0 {
		t.Errorf("zones = %+v", zones)
	}
	if paths := zones[1].SuggestedPaths(); len(paths) != 1 || paths[0].String() != "internal/ports/**" {
		t.Errorf("ports suggested paths = %v", paths)
	}
	if exts := p.Extensions(); len(exts) != 1 || exts[0].FileName() != "acme_check.ts" {
		t.Errorf("extensions = %+v", exts)
	}
	if _, err := yamlrule.LoadPattern([]byte("zones:\n  core: \"desc\"\n"), "headerless.yaml", nil); err == nil ||
		!strings.Contains(err.Error(), "missing pattern header") {
		t.Errorf("headerless file: %v", err)
	}
	for name, tc := range map[string]struct{ document, want string }{
		"namespaced rule id": {`
pattern: {namespace: acme, name: hexagonal, version: 1.0.0}
zones:
  core: "The core."
rules:
  acme/hexagonal:core/stdlib-only:
    on: core
    imports:
      internal: []
`, "rule ids inside a pattern are local"},
		"override inside a pattern": {`
pattern: {namespace: acme, name: hexagonal, version: 1.0.0}
zones:
  core: "The core."
rules:
  core/stdlib-only:
    severity: warning
`, "a pattern distributes Rules and cannot override"},
		"zone without description": {`
pattern: {namespace: acme, name: hexagonal, version: 1.0.0}
zones:
  core:
    paths: ["core/**"]
`, "a pattern zone requires a description"},
		"rule naming an unlisted zone": {`
pattern: {namespace: acme, name: hexagonal, version: 1.0.0}
zones:
  core: "The core."
rules:
  core/imports:
    on: core
    imports:
      internal: [shared]
`, `names zone "shared", which is not declared`},
		"pattern without rules": {`
pattern: {namespace: acme, name: hexagonal, version: 1.0.0}
zones:
  core: "The core."
`, "no rules"},
		"coverage spelled as a language name": {`
pattern: {namespace: acme, name: hexagonal, version: 1.0.0, coverage: [typescript]}
zones:
  core: "The core."
rules:
  core/stdlib-only:
    on: core
    imports:
      internal: []
`, `pattern.coverage target "typescript": not one of go, ts, py`},
		"coverage listed twice": {`
pattern: {namespace: acme, name: hexagonal, version: 1.0.0, coverage: [go, go]}
zones:
  core: "The core."
rules:
  core/stdlib-only:
    on: core
    imports:
      internal: []
`, `pattern.coverage target "go": listed twice`},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := yamlrule.LoadPattern([]byte(tc.document), "pattern.yaml", nil)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}

// TestLoadExtendsBindsAndOverrides proves a repository ruleset adopts a
// Pattern: every listed Zone is bound, the Pattern's Rules arrive
// first under their qualified ids, Overrides merge onto them, local
// Rules follow, and the Pattern's Extensions are supplied.
func TestLoadExtendsBindsAndOverrides(t *testing.T) {
	p := loadSamplePattern(t)
	cfg := mustLoad(t, `
runtime: [go]
extends:
  - pattern: acme/hexagonal@1.0.0
    bind:
      core: internal/core/**
      ports: ["internal/ports/**", "internal/api/**"]
      adapters:
        - internal/adapters/**
zones:
  shared: internal/shared/**
rules:
  acme/hexagonal:core/stdlib-only:
    severity: warning
  acme/hexagonal:core/no-panic:
    disable: "the core wraps a legacy library that panics"
  acme/hexagonal:core/checked:
    exclude:
      paths: ["internal/core/generated/**"]
      reason: "generated"
    suppress:
      paths: ["internal/core/legacy/**"]
      reason: "adopted debt"
  shared/imports:
    on: shared
    imports:
      internal: [core]
`, p)
	if len(cfg.Zones) != 4 {
		t.Fatalf("zones = %d, want 3 bound + 1 local", len(cfg.Zones))
	}
	core := cfg.Zones[0]
	if core.Name() != "core" || core.Description() != "The domain core." || !core.Contains("internal/core/x.go") {
		t.Errorf("bound core = %+v", core)
	}
	if ports := cfg.Zones[1]; len(ports.Paths()) != 2 {
		t.Errorf("bound ports paths = %v", ports.Paths())
	}
	if cfg.Zones[3].Name() != "shared" {
		t.Errorf("local zone must follow the bound ones, got %s", cfg.Zones[3].Name())
	}
	ids := ruleIDs(cfg)
	if len(ids) != 7 || ids[0] != "acme/hexagonal:core/stdlib-only" || ids[6] != "shared/imports" {
		t.Fatalf("rules = %v, want the six pattern rules then the local one", ids)
	}
	if r := ruleByID(t, cfg, "acme/hexagonal:core/stdlib-only"); r.Severity() != rule.SeverityWarning {
		t.Errorf("override severity = %s", r.Severity())
	} else if r.Rationale().String() != "The core imports no other Zone and no third-party package." {
		t.Errorf("an override keeps the pattern's rationale, got %q", r.Rationale())
	}
	if r := ruleByID(t, cfg, "acme/hexagonal:core/no-panic"); !r.Disabled() {
		t.Errorf("override disable must disable the pattern rule")
	}
	checked := ruleByID(t, cfg, "acme/hexagonal:core/checked")
	if checked.AppliesToFile("internal/core/generated/x.go", []rule.ZoneName{"core"}) {
		t.Errorf("override exclude must narrow the pattern rule")
	}
	if _, ok := checked.SuppressionFor("internal/core/legacy/x.go"); !ok {
		t.Errorf("override suppress must suppress the pattern rule's findings")
	}
	if untouched := ruleByID(t, cfg, "acme/hexagonal:adapters/core-only"); untouched.Severity() != rule.SeverityError {
		t.Errorf("an unoverridden pattern rule keeps its severity")
	}
	if len(cfg.Extensions) != 1 || cfg.Extensions[0].Pattern.String() != "acme/hexagonal@1.0.0" ||
		cfg.Extensions[0].Extension.FileName() != "acme_check.ts" {
		t.Errorf("extensions = %+v", cfg.Extensions)
	}
}

// TestLoadRejectsMalformedAdoption pins the errors an adopting
// repository meets: unbound or unknown Zones, colliding Zones,
// Overrides of nothing, and Rules that try to redefine a Pattern Rule.
func TestLoadRejectsMalformedAdoption(t *testing.T) {
	p := loadSamplePattern(t)
	const bound = `
extends:
  - pattern: acme/hexagonal@1.0.0
    bind:
      core: internal/core/**
      ports: internal/ports/**
      adapters: internal/adapters/**
`
	cases := map[string]struct{ document, want string }{
		"unbound zone": {`
extends:
  - pattern: acme/hexagonal@1.0.0
    bind:
      core: internal/core/**
`, "unbound zones ports, adapters"},
		"bind of an unlisted zone": {bound + `      extra: internal/extra/**
`, "bind names zones the pattern does not list: extra"},
		"pattern extended twice": {bound + `  - pattern: acme/hexagonal@1.0.0
    bind:
      core: internal/core/**
      ports: internal/ports/**
      adapters: internal/adapters/**
`, "extended twice"},
		"local zone collides with a bound one": {bound + `zones:
  core: internal/other/**
`, `zone "core" is already bound by an extended pattern`},
		"override of an undistributed rule": {bound + `rules:
  acme/hexagonal:core/missing:
    severity: warning
`, "no extended pattern or built-in rule distributes rule acme/hexagonal:core/missing"},
		"override of an unqualified id": {bound + `rules:
  core/stdlib-only:
    severity: warning
`, "no extended pattern or built-in rule distributes rule core/stdlib-only"},
		"override with description": {bound + `rules:
  acme/hexagonal:core/stdlib-only:
    description: "rewritten"
    severity: warning
`, `unknown key "description"`},
		"override with on": {bound + `rules:
  acme/hexagonal:core/stdlib-only:
    on: ports
    severity: warning
`, "an override does not accept on"},
		"override with files": {bound + `rules:
  acme/hexagonal:core/no-panic:
    files: "**/*.go"
    severity: warning
`, "an override does not accept files"},
		"override with with": {bound + `rules:
  acme/hexagonal:core/checked:
    with:
      depth: 1
    severity: warning
`, "an override does not accept with"},
		"override without change": {bound + `rules:
  acme/hexagonal:core/stdlib-only: {}
`, "an override changes something"},
		"local rule redefines a pattern rule": {bound + `rules:
  acme/hexagonal:core/stdlib-only:
    on: core
    imports:
      internal: [ports]
`, "is distributed by an extended pattern"},
		"override twice": {bound + `rules:
  acme/hexagonal:core/stdlib-only:
    severity: warning
  acme/hexagonal:core/stdlib-only:
    severity: info
`, `appears twice`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := loadString(t, tc.document, p)
			if err == nil {
				t.Fatalf("expected a load error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to mention %q", err, tc.want)
			}
		})
	}
}

// TestLoadAppliesOneLocalIdFromTwoPatterns proves two Patterns may
// distribute the same local Rule id into one repository: each Rule
// applies under its own namespace/name qualifier, an Override reaches
// exactly one of them, and every finding still names its Pattern.
func TestLoadAppliesOneLocalIdFromTwoPatterns(t *testing.T) {
	p := loadSamplePattern(t)
	other, err := yamlrule.LoadPattern([]byte(`
pattern:
  namespace: acme
  name: onion
  version: 2.0.0
zones:
  core: "The core, again."
rules:
  core/stdlib-only:
    on: core
    imports:
      internal: []
`), "acme/onion/pattern.yaml", nil)
	if err != nil {
		t.Fatalf("LoadPattern: %v", err)
	}
	cfg := mustLoad(t, `
extends:
  - pattern: acme/hexagonal@1.0.0
    bind:
      core: internal/core/**
      ports: internal/ports/**
      adapters: internal/adapters/**
  - pattern: acme/onion@2.0.0
    bind:
      core: internal/core/**
rules:
  acme/onion:core/stdlib-only:
    severity: warning
`, p, other)
	hex := ruleByID(t, cfg, "acme/hexagonal:core/stdlib-only")
	onion := ruleByID(t, cfg, "acme/onion:core/stdlib-only")
	if hex.ID().Local() != onion.ID().Local() || hex.ID().Equals(onion.ID()) {
		t.Errorf("ids %s and %s must share a local identity and differ as RuleIDs", hex.ID(), onion.ID())
	}
	if hex.Severity() != rule.SeverityError || onion.Severity() != rule.SeverityWarning {
		t.Errorf("override reached hex=%s onion=%s, want error and warning", hex.Severity(), onion.Severity())
	}
	hexRef, _ := hex.Provenance()
	onionRef, _ := onion.Provenance()
	if hexRef.String() != "acme/hexagonal@1.0.0" || onionRef.String() != "acme/onion@2.0.0" {
		t.Errorf("provenance hex=%s onion=%s", hexRef, onionRef)
	}
	_, err = loadString(t, `
extends:
  - pattern: acme/onion@2.0.0
    bind:
      core: internal/elsewhere/**
  - pattern: acme/hexagonal@1.0.0
    bind:
      core: internal/core/**
      ports: internal/ports/**
      adapters: internal/adapters/**
`, p, other)
	if err == nil || !strings.Contains(err.Error(), `zone "core" is already declared with different paths`) {
		t.Errorf("error = %v", err)
	}
}

// TestLoadScopesPatternAcyclicToItsZones proves a Pattern's
// `acyclic: {}` names the Pattern's own Zones and nothing the
// adopting repository or a sibling Pattern declares, while the
// repository's own `{}` stays open over every declared Zone.
func TestLoadScopesPatternAcyclicToItsZones(t *testing.T) {
	p := loadSamplePattern(t)
	if got := ruleByID(t, mustLoad(t, `
extends:
  - pattern: acme/hexagonal@1.0.0
    bind:
      core: internal/core/**
      ports: internal/ports/**
      adapters: internal/adapters/**
zones:
  source:
    paths: "internal/**"
rules:
  repo/acyclic:
    acyclic: {}
`, p), "acme/hexagonal:deps/acyclic").Params().(rule.AcyclicParams).Zones; !reflect.DeepEqual(got, []rule.ZoneName{"core", "ports", "adapters"}) {
		t.Errorf("a Pattern's acyclic {} must name the Pattern's Zones in order, got %v", got)
	}
	if got := ruleByID(t, mustLoad(t, `
extends:
  - pattern: acme/hexagonal@1.0.0
    bind:
      core: internal/core/**
      ports: internal/ports/**
      adapters: internal/adapters/**
zones:
  source:
    paths: "internal/**"
rules:
  repo/acyclic:
    acyclic: {}
`, p), "repo/acyclic").Params().(rule.AcyclicParams).Zones; len(got) != 0 {
		t.Errorf("the repository's acyclic {} stays open over every declared Zone, got %v", got)
	}
}

// TestLoadRejectsOnePatternAtTwoVersions proves the qualifier is the
// unit of extension: a Pattern extended at two versions would
// distribute every Rule id twice, so the loader names the first.
func TestLoadRejectsOnePatternAtTwoVersions(t *testing.T) {
	p := loadSamplePattern(t)
	newer, err := yamlrule.LoadPattern([]byte(strings.Replace(samplePatternFile, "version: 1.0.0", "version: 1.1.0", 1)),
		"acme/hexagonal/pattern.yaml", nil)
	if err != nil {
		t.Fatalf("LoadPattern: %v", err)
	}
	_, err = loadString(t, `
extends:
  - pattern: acme/hexagonal@1.0.0
    bind:
      core: internal/core/**
      ports: internal/ports/**
      adapters: internal/adapters/**
  - pattern: acme/hexagonal@1.1.0
    bind:
      core: internal/core/**
      ports: internal/ports/**
      adapters: internal/adapters/**
`, p, newer)
	if err == nil || !strings.Contains(err.Error(), "pattern acme/hexagonal@1.1.0 is already extended as acme/hexagonal@1.0.0") {
		t.Errorf("error = %v", err)
	}
}

// TestRepositoryResolvesPatternsFromSources proves the Repository port
// gathers available Patterns only when the ruleset extends one, and
// refuses a Pattern file in the ruleset's place.
func TestRepositoryResolvesPatternsFromSources(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "rules.arclint.yaml")
	if err := os.WriteFile(file, []byte(samplePatternFile), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	repo, err := yamlrule.NewRepository(file, nil)
	if err != nil {
		t.Fatalf("NewRepository: %v", err)
	}
	if _, err := repo.ConfiguredRules(); err == nil || !strings.Contains(err.Error(), "a pattern distribution file is not a repository ruleset") {
		t.Errorf("a pattern distribution file must not load as a repository ruleset: %v", err)
	}
	if _, err := yamlrule.NewRepository(file, nil, nil); err == nil {
		t.Errorf("a nil pattern source must be rejected")
	}

	source := &countingSource{patterns: []rule.Pattern{loadSamplePattern(t)}}
	adopting := filepath.Join(dir, "adopting.yaml")
	if err := os.WriteFile(adopting, []byte(`
runtime: [go]
extends:
  - pattern: acme/hexagonal@1.0.0
    bind:
      core: internal/core/**
      ports: internal/ports/**
      adapters: internal/adapters/**
`), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	repo, err = yamlrule.NewRepository(adopting, nil, source)
	if err != nil {
		t.Fatalf("NewRepository: %v", err)
	}
	cfg, err := repo.ConfiguredRules()
	if err != nil {
		t.Fatalf("ConfiguredRules: %v", err)
	}
	if _, err := repo.ConfiguredRules(); err != nil {
		t.Fatalf("ConfiguredRules (memoized): %v", err)
	}
	if source.calls != 1 {
		t.Errorf("pattern sources consulted %d times, want once", source.calls)
	}
	if len(cfg.Rules) != 6 || len(cfg.Zones) != 3 {
		t.Errorf("adopted %d rules and %d zones", len(cfg.Rules), len(cfg.Zones))
	}

	bare := filepath.Join(dir, "bare.yaml")
	if err := os.WriteFile(bare, []byte("runtime: [go]\nzones:\n  m: m/**\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	source = &countingSource{}
	repo, err = yamlrule.NewRepository(bare, nil, source)
	if err != nil {
		t.Fatalf("NewRepository: %v", err)
	}
	if _, err := repo.ConfiguredRules(); err != nil {
		t.Fatalf("ConfiguredRules: %v", err)
	}
	if source.calls != 0 {
		t.Errorf("a ruleset without extends must not consult pattern sources")
	}
}

type countingSource struct {
	patterns []rule.Pattern
	calls    int
}

func (s *countingSource) Patterns() ([]rule.Pattern, error) {
	s.calls++
	return s.patterns, nil
}

func TestDiscoverPath(t *testing.T) {
	root := t.TempDir()
	nested := root + "/a/b"
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	target := root + "/rules.arclint.yaml"
	if err := os.WriteFile(target, []byte("runtime: [go]\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	found, err := yamlrule.DiscoverPath(nested, "rules.arclint.yaml")
	if err != nil {
		t.Fatalf("DiscoverPath: %v", err)
	}
	if found != target {
		t.Errorf("DiscoverPath = %q, want %q", found, target)
	}
	if _, err := yamlrule.DiscoverPath(t.TempDir(), "rules.arclint.yaml"); err == nil {
		t.Errorf("expected discovery failure in an empty tree")
	}
}
