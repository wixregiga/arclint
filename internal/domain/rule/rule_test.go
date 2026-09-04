package rule_test

import (
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/rule"
)

func mustModuleApplicability(t *testing.T, names ...string) rule.Applicability {
	t.Helper()
	modules := make([]rule.ModuleName, 0, len(names))
	for _, n := range names {
		m, err := rule.NewModuleName(n)
		if err != nil {
			t.Fatalf("NewModuleName(%q): %v", n, err)
		}
		modules = append(modules, m)
	}
	a, err := rule.ModuleApplicability(modules)
	if err != nil {
		t.Fatalf("ModuleApplicability(%v): %v", names, err)
	}
	return a
}

func mustRepoApplicability(t *testing.T) rule.Applicability {
	t.Helper()
	a, err := rule.RepositoryApplicability()
	if err != nil {
		t.Fatalf("RepositoryApplicability: %v", err)
	}
	return a
}

func emptyAllowList(t *testing.T) *rule.AllowList {
	t.Helper()
	l, err := rule.NewAllowList()
	if err != nil {
		t.Fatalf("NewAllowList: %v", err)
	}
	return &l
}

func validConsumesSpec(t *testing.T) rule.Spec {
	t.Helper()
	return rule.Spec{
		ID:            "arclint:domain/stdlib-only",
		Type:          rule.TypeConsumes,
		Params:        rule.ConsumesParams{Internal: emptyAllowList(t), External: rule.ImportForbid},
		Applicability: mustModuleApplicability(t, "domain"),
	}
}

func TestInvalidRulesCannotBeConstructed(t *testing.T) {
	snake, err := rule.NewCaseSpec("snake_case")
	if err != nil {
		t.Fatalf("NewCaseSpec: %v", err)
	}
	cases := []struct {
		name string
		spec rule.Spec
	}{
		{"empty id", func() rule.Spec { s := validConsumesSpec(t); s.ID = ""; return s }()},
		{"unpublished type", func() rule.Spec { s := validConsumesSpec(t); s.Type = "correspondence"; return s }()},
		{"params mismatch", func() rule.Spec {
			s := validConsumesSpec(t)
			s.Params = rule.NamingParams{Case: snake}
			return s
		}()},
		{"invalid severity", func() rule.Spec { s := validConsumesSpec(t); s.Severity = "warn"; return s }()},
		{"consumes without restriction", func() rule.Spec {
			s := validConsumesSpec(t)
			s.Params = rule.ConsumesParams{}
			return s
		}()},
		{"consumes without module scope", func() rule.Spec {
			s := validConsumesSpec(t)
			s.Applicability = mustRepoApplicability(t)
			return s
		}()},
		{"layers with one layer", rule.Spec{
			ID:            "t:one-layer",
			Type:          rule.TypeLayers,
			Params:        rule.LayersParams{Layers: []rule.ModuleName{"a"}},
			Applicability: mustRepoApplicability(t),
		}},
		{"layers with module scope", rule.Spec{
			ID:            "t:layers-scope",
			Type:          rule.TypeLayers,
			Params:        rule.LayersParams{Layers: []rule.ModuleName{"a", "b"}},
			Applicability: mustModuleApplicability(t, "a"),
		}},
		{"structure without globs", rule.Spec{
			ID:            "t:structure-empty",
			Type:          rule.TypeStructure,
			Params:        rule.StructureParams{},
			Applicability: mustModuleApplicability(t, "a"),
		}},
		{"naming without case", rule.Spec{
			ID:            "t:naming-empty",
			Type:          rule.TypeNaming,
			Params:        rule.NamingParams{},
			Applicability: mustModuleApplicability(t, "a"),
		}},
		{"independence without folders", rule.Spec{
			ID:            "t:independence-empty",
			Type:          rule.TypeIndependence,
			Params:        rule.IndependenceParams{},
			Applicability: mustRepoApplicability(t),
		}},
		{"independence with duplicate folders", rule.Spec{
			ID:            "t:independence-dup",
			Type:          rule.TypeIndependence,
			Params:        rule.IndependenceParams{Folders: []rule.Glob{mustGlob(t, "internal/*"), mustGlob(t, "internal/*")}},
			Applicability: mustRepoApplicability(t),
		}},
		{"independence with module scope", rule.Spec{
			ID:            "t:independence-scope",
			Type:          rule.TypeIndependence,
			Params:        rule.IndependenceParams{Folders: []rule.Glob{mustGlob(t, "internal/*")}},
			Applicability: mustModuleApplicability(t, "a"),
		}},
	}
	for _, c := range cases {
		if _, err := rule.New(c.spec); err == nil {
			t.Errorf("%s: expected construction error", c.name)
		}
	}
}

func TestDerivedClaim(t *testing.T) {
	r, err := rule.New(validConsumesSpec(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	claim := r.Claim().Statement()
	for _, want := range []string{`Module "domain"`, "no other declared Module", "no external imports"} {
		if !strings.Contains(claim, want) {
			t.Errorf("derived claim %q lacks %q", claim, want)
		}
	}
	if r.Severity() != rule.SeverityError {
		t.Errorf("default severity = %q, want error", r.Severity())
	}
	if !r.Enforcement().CanEvaluate() {
		t.Errorf("builtin consumes enforcement must be evaluable")
	}
}

func TestConfigurationPreservesIdentity(t *testing.T) {
	r, err := rule.New(validConsumesSpec(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	configured, err := r.WithSeverity(rule.SeverityWarning)
	if err != nil {
		t.Fatalf("WithSeverity: %v", err)
	}
	if !configured.ID().Equals(r.ID()) {
		t.Errorf("configuration changed identity")
	}
	if r.Severity() != rule.SeverityError {
		t.Errorf("original rule mutated: immutability broken")
	}

	glob, err := rule.NewGlob("internal/domain/legacy/**")
	if err != nil {
		t.Fatalf("NewGlob: %v", err)
	}
	exclusion, err := rule.NewExclusion([]rule.Glob{glob}, nil, "legacy code adopted as-is")
	if err != nil {
		t.Fatalf("NewExclusion: %v", err)
	}
	excluded := r.Exclude(exclusion)
	member := []rule.ModuleName{"domain"}
	if excluded.AppliesToFile("internal/domain/legacy/x.go", member) {
		t.Errorf("excluded subject still selected")
	}
	if !excluded.AppliesToFile("internal/domain/rule/root.go", member) {
		t.Errorf("exclusion removed an unselected subject")
	}
	if r.AppliesToFile("internal/domain/legacy/x.go", member) != true {
		t.Errorf("original rule mutated by Exclude")
	}

	suppression, err := rule.NewSuppression([]rule.Glob{glob}, "known debt")
	if err != nil {
		t.Fatalf("NewSuppression: %v", err)
	}
	suppressed := r.Suppress(suppression)
	if reason, ok := suppressed.SuppressionFor("internal/domain/legacy/x.go"); !ok || reason != "known debt" {
		t.Errorf("SuppressionFor = (%q, %v), want (known debt, true)", reason, ok)
	}

	disablement, err := rule.NewDisablement("rule retired for this repository")
	if err != nil {
		t.Fatalf("NewDisablement: %v", err)
	}
	disabled := r.Disable(disablement)
	if !disabled.Disabled() || r.Disabled() {
		t.Errorf("Disable must produce a disabled copy and leave the original enabled")
	}
}

func TestIDQualification(t *testing.T) {
	id, err := rule.NewID("arclint:domain/stdlib-only")
	if err != nil {
		t.Fatalf("NewID: %v", err)
	}
	if id.Namespace() != "arclint" || id.Local() != "domain/stdlib-only" {
		t.Errorf("parsed id = %q/%q", id.Namespace(), id.Local())
	}
	if id.Qualified() != "arclint:domain/stdlib-only" {
		t.Errorf("Qualified = %q", id.Qualified())
	}
	local, err := rule.NewID("repo-rule")
	if err != nil {
		t.Fatalf("NewID: %v", err)
	}
	if local.Qualified() != "repo-rule" {
		t.Errorf("local Qualified = %q", local.Qualified())
	}
	for _, bad := range []string{"", ":x", "a:", "a:b:c", "UPPER", "a b", ".start"} {
		if _, err := rule.NewID(bad); err == nil {
			t.Errorf("NewID(%q): expected error", bad)
		}
	}
}

func TestCaseSpec(t *testing.T) {
	spec, err := rule.NewCaseSpec("snake_case|regex:^v[0-9]+$")
	if err != nil {
		t.Fatalf("NewCaseSpec: %v", err)
	}
	for stem, want := range map[string]bool{
		"good_name": true,
		"v12":       true,
		"BadName":   false,
	} {
		if spec.Matches(stem) != want {
			t.Errorf("Matches(%q) = %v, want %v", stem, !want, want)
		}
	}
	for _, bad := range []string{"", "SCREAMING_CASE", "regex:("} {
		if _, err := rule.NewCaseSpec(bad); err == nil {
			t.Errorf("NewCaseSpec(%q): expected error", bad)
		}
	}
}

func mustPatternModule(t *testing.T, name, description string, paths ...string) rule.PatternModule {
	t.Helper()
	n, err := rule.NewModuleName(name)
	if err != nil {
		t.Fatalf("NewModuleName(%q): %v", name, err)
	}
	globs, err := rule.NewGlobs(paths)
	if err != nil {
		t.Fatalf("NewGlobs(%v): %v", paths, err)
	}
	m, err := rule.NewPatternModule(n, description, globs)
	if err != nil {
		t.Fatalf("NewPatternModule(%q): %v", name, err)
	}
	return m
}

func mustBinding(t *testing.T, name string, paths ...string) rule.Binding {
	t.Helper()
	n, err := rule.NewModuleName(name)
	if err != nil {
		t.Fatalf("NewModuleName(%q): %v", name, err)
	}
	globs, err := rule.NewGlobs(paths)
	if err != nil {
		t.Fatalf("NewGlobs(%v): %v", paths, err)
	}
	b, err := rule.NewBinding(n, globs)
	if err != nil {
		t.Fatalf("NewBinding(%q): %v", name, err)
	}
	return b
}

func validPatternSpec(t *testing.T) rule.PatternSpec {
	t.Helper()
	r, err := rule.New(validConsumesSpec(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return rule.PatternSpec{
		Namespace: "arclint",
		Name:      "ddd-flat",
		Version:   "1.0.0",
		Coverage:  []rule.Language{rule.LanguageGo},
		Modules:   []rule.PatternModule{mustPatternModule(t, "domain", "The model.", "internal/domain/**")},
		Rules:     []rule.Rule{r},
	}
}

func TestPatternReferenceParsing(t *testing.T) {
	ref, err := rule.ParsePatternReference(" arclint/ddd-flat@1.2.3-rc.1 ")
	if err != nil {
		t.Fatalf("ParsePatternReference: %v", err)
	}
	if ref.Namespace() != "arclint" || ref.Name() != "ddd-flat" || ref.Version() != "1.2.3-rc.1" ||
		ref.String() != "arclint/ddd-flat@1.2.3-rc.1" || ref.IsZero() {
		t.Errorf("reference = %+v", ref)
	}
	for _, bad := range []string{
		"", "ddd-flat", "ddd-flat@1.0.0", "arclint/ddd-flat", "arclint/ddd-flat@latest",
		"arclint/ddd-flat@1", "a/b/c@1.0.0", "Arclint/ddd-flat@1.0.0", "/ddd-flat@1.0.0",
	} {
		if _, err := rule.ParsePatternReference(bad); err == nil {
			t.Errorf("ParsePatternReference(%q): expected error", bad)
		}
	}
	if !(rule.PatternReference{}).IsZero() {
		t.Errorf("zero reference must report IsZero")
	}
}

func TestPatternModuleAndBinding(t *testing.T) {
	m := mustPatternModule(t, "domain", "  The model.  ", "internal/domain/**")
	if m.Name().String() != "domain" || m.Description() != "The model." {
		t.Errorf("module = %+v", m)
	}
	if paths := m.SuggestedPaths(); len(paths) != 1 || paths[0].String() != "internal/domain/**" {
		t.Errorf("suggested paths = %v", paths)
	}
	paths := m.SuggestedPaths()
	paths[0] = rule.Glob{}
	if again := m.SuggestedPaths(); again[0].IsZero() {
		t.Errorf("SuggestedPaths must return a copy")
	}
	if _, err := rule.NewPatternModule("domain", "   ", nil); err == nil {
		t.Errorf("a pattern module without a description must be rejected")
	}
	if _, err := rule.NewPatternModule("Bad Name", "desc", nil); err == nil {
		t.Errorf("an invalid module name must be rejected")
	}
	if _, err := rule.NewPatternModule("domain", "desc", []rule.Glob{{}}); err == nil {
		t.Errorf("an unconstructed suggested path must be rejected")
	}
	b := mustBinding(t, "domain", "src/domain/**", "lib/domain/**")
	if b.Module().String() != "domain" || len(b.Paths()) != 2 {
		t.Errorf("binding = %+v", b)
	}
	if _, err := rule.NewBinding("domain", nil); err == nil {
		t.Errorf("a binding without paths must be rejected")
	}
	if _, err := rule.NewBinding("domain", []rule.Glob{{}}); err == nil {
		t.Errorf("an unconstructed bound path must be rejected")
	}
}

func TestPatternStampsProvenance(t *testing.T) {
	spec := validPatternSpec(t)
	spec.Documentation = "  https://example.test/ddd-flat  "
	p, err := rule.NewPattern(spec)
	if err != nil {
		t.Fatalf("NewPattern: %v", err)
	}
	carried := p.Rules()[0]
	ref, ok := carried.Provenance()
	if !ok || ref.String() != "arclint/ddd-flat@1.0.0" {
		t.Errorf("provenance = %v %v", ref, ok)
	}
	if p.Documentation() != "https://example.test/ddd-flat" {
		t.Errorf("documentation = %q", p.Documentation())
	}
	if mods := p.Modules(); len(mods) != 1 || mods[0].Name().String() != "domain" {
		t.Errorf("modules = %+v", mods)
	}
	if cov := p.Coverage(); len(cov) != 1 || cov[0] != rule.LanguageGo {
		t.Errorf("coverage = %v", cov)
	}
}

func TestPatternRejectsMalformedSpecs(t *testing.T) {
	r, err := rule.New(validConsumesSpec(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	foreign, err := rule.New(rule.Spec{
		ID:            "other:domain/stdlib-only",
		Type:          rule.TypeConsumes,
		Params:        rule.ConsumesParams{Internal: emptyAllowList(t), External: rule.ImportForbid},
		Applicability: mustModuleApplicability(t, "domain"),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	cases := map[string]func(*rule.PatternSpec){
		"duplicate rule ids":             func(s *rule.PatternSpec) { s.Rules = []rule.Rule{r, r} },
		"inexact version":                func(s *rule.PatternSpec) { s.Version = "latest" },
		"no rules":                       func(s *rule.PatternSpec) { s.Rules = nil },
		"unconstructed rule":             func(s *rule.PatternSpec) { s.Rules = []rule.Rule{{}} },
		"rule outside the namespace":     func(s *rule.PatternSpec) { s.Rules = []rule.Rule{foreign} },
		"rule naming an unlisted module": func(s *rule.PatternSpec) { s.Modules = nil },
		"duplicate module":               func(s *rule.PatternSpec) { s.Modules = append(s.Modules, s.Modules[0]) },
		"unconstructed module":           func(s *rule.PatternSpec) { s.Modules = []rule.PatternModule{{}} },
		"invalid coverage":               func(s *rule.PatternSpec) { s.Coverage = []rule.Language{"cobol"} },
		"unconstructed extension":        func(s *rule.PatternSpec) { s.Extensions = []rule.PatternExtension{{}} },
	}
	for name, mutate := range cases {
		spec := validPatternSpec(t)
		mutate(&spec)
		if _, err := rule.NewPattern(spec); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestPatternBind(t *testing.T) {
	spec := validPatternSpec(t)
	spec.Modules = append(spec.Modules, mustPatternModule(t, "application", "Use cases."))
	p, err := rule.NewPattern(spec)
	if err != nil {
		t.Fatalf("NewPattern: %v", err)
	}
	bound, err := p.Bind([]rule.Binding{
		mustBinding(t, "application", "internal/application/**"),
		mustBinding(t, "domain", "internal/domain/**", "pkg/model/**"),
	})
	if err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if len(bound) != 2 || bound[0].Name().String() != "domain" || bound[1].Name().String() != "application" {
		t.Fatalf("Bind must follow the pattern's declared order; got %+v", bound)
	}
	if bound[0].Description() != "The model." || len(bound[0].Paths()) != 2 || !bound[0].Contains("pkg/model/x.go") {
		t.Errorf("bound domain = %+v", bound[0])
	}
	if _, err := p.Bind([]rule.Binding{mustBinding(t, "domain", "internal/domain/**")}); err == nil ||
		!strings.Contains(err.Error(), "unbound modules application") {
		t.Errorf("unbound module: got %v", err)
	}
	if _, err := p.Bind([]rule.Binding{
		mustBinding(t, "domain", "a/**"), mustBinding(t, "application", "b/**"), mustBinding(t, "shared", "c/**"),
	}); err == nil || !strings.Contains(err.Error(), "does not list: shared") {
		t.Errorf("unknown binding: got %v", err)
	}
	if _, err := p.Bind([]rule.Binding{
		mustBinding(t, "domain", "a/**"), mustBinding(t, "domain", "b/**"), mustBinding(t, "application", "c/**"),
	}); err == nil || !strings.Contains(err.Error(), "bound twice") {
		t.Errorf("double binding: got %v", err)
	}
}

func TestPatternExtensionValidation(t *testing.T) {
	if _, err := rule.NewPatternExtension("", "export default {}"); err == nil {
		t.Errorf("empty file name must be rejected")
	}
	for _, name := range []string{"a/b.ts", `a\b.ts`, ".", "..", ".hidden.ts", "types.d.ts", "readme.md"} {
		if _, err := rule.NewPatternExtension(name, "export default {}"); err == nil {
			t.Errorf("NewPatternExtension(%q): expected error", name)
		}
	}
	if _, err := rule.NewPatternExtension("ok.ts", ""); err == nil {
		t.Errorf("blank source must be rejected")
	}
	if _, err := rule.NewPatternExtension("ok.ts", "   \n"); err == nil {
		t.Errorf("whitespace-only source must be rejected")
	}
	e, err := rule.NewPatternExtension("ok.ts", "export default {}")
	if err != nil {
		t.Fatalf("NewPatternExtension: %v", err)
	}
	if e.FileName() != "ok.ts" || e.Source() != "export default {}" {
		t.Errorf("extension = %+v", e)
	}
}

func TestPatternRejectsDuplicateExtensions(t *testing.T) {
	e, err := rule.NewPatternExtension("ok.ts", "export default {}")
	if err != nil {
		t.Fatalf("NewPatternExtension: %v", err)
	}
	spec := validPatternSpec(t)
	spec.Extensions = []rule.PatternExtension{e, e}
	if _, err := rule.NewPattern(spec); err == nil {
		t.Errorf("duplicate extension filenames in a pattern must be rejected")
	}
}

func TestPatternExtensionsReturnsCopy(t *testing.T) {
	e, err := rule.NewPatternExtension("ok.ts", "export default {}")
	if err != nil {
		t.Fatalf("NewPatternExtension: %v", err)
	}
	spec := validPatternSpec(t)
	spec.Extensions = []rule.PatternExtension{e}
	p, err := rule.NewPattern(spec)
	if err != nil {
		t.Fatalf("NewPattern: %v", err)
	}
	got := p.Extensions()
	if len(got) != 1 || got[0].FileName() != "ok.ts" {
		t.Fatalf("Extensions = %+v", got)
	}
	got[0] = rule.PatternExtension{}
	again := p.Extensions()
	if len(again) != 1 || again[0].FileName() != "ok.ts" {
		t.Errorf("Extensions must return a copy; got %+v", again)
	}
}

func TestAssertionKeysSpellEveryType(t *testing.T) {
	keys := rule.AssertionKeys()
	if len(keys) != len(rule.Types()) {
		t.Fatalf("AssertionKeys = %v, want one per Type", keys)
	}
	for i, typ := range rule.Types() {
		if typ.AssertionKey() != keys[i] {
			t.Errorf("%s: AssertionKey %q != AssertionKeys()[%d] %q", typ, typ.AssertionKey(), i, keys[i])
		}
		back, ok := rule.TypeOfAssertionKey(keys[i])
		if !ok || back != typ {
			t.Errorf("TypeOfAssertionKey(%q) = %v %v, want %s", keys[i], back, ok, typ)
		}
	}
	for key, want := range map[string]rule.Type{
		"imports": rule.TypeConsumes, "imported_by": rule.TypeProtected,
		"independent": rule.TypeIndependence, "uses": rule.TypeExtension, "content": rule.TypeContent,
	} {
		if got, ok := rule.TypeOfAssertionKey(key); !ok || got != want {
			t.Errorf("TypeOfAssertionKey(%q) = %v %v, want %s", key, got, ok, want)
		}
	}
	if _, ok := rule.TypeOfAssertionKey("kind"); ok {
		t.Errorf("a retired key must not resolve to a Type")
	}
}

func TestTypeScopeAndFiles(t *testing.T) {
	for typ, want := range map[rule.Type]rule.Scope{
		rule.TypeConsumes: rule.ScopeModules, rule.TypeStructure: rule.ScopeModules,
		rule.TypeNaming: rule.ScopeModules, rule.TypeInvariants: rule.ScopeModules,
		rule.TypeProtected: rule.ScopeOneModule,
		rule.TypeLayers:    rule.ScopeRepository, rule.TypeIndependence: rule.ScopeRepository,
		rule.TypeAcyclic: rule.ScopeRepository,
		rule.TypeContent: rule.ScopeModulesOrRepository, rule.TypeExtension: rule.ScopeModulesOrRepository,
	} {
		if got := typ.Scope(); got != want {
			t.Errorf("%s.Scope() = %v, want %v", typ, got, want)
		}
	}
	for typ, want := range map[rule.Type]bool{
		rule.TypeNaming: true, rule.TypeContent: true, rule.TypeExtension: true,
		rule.TypeConsumes: false, rule.TypeStructure: false, rule.TypeLayers: false,
	} {
		if got := typ.AcceptsFiles(); got != want {
			t.Errorf("%s.AcceptsFiles() = %v, want %v", typ, got, want)
		}
	}
}

func TestContentParams(t *testing.T) {
	r, err := rule.New(rule.Spec{
		ID:            "domain/no-panic",
		Type:          rule.TypeContent,
		Params:        rule.ContentParams{Forbid: `\bpanic\(`},
		Applicability: mustModuleApplicability(t, "domain"),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if !strings.Contains(r.Claim().String(), `contains no line matching /\bpanic\(/`) {
		t.Errorf("claim = %q", r.Claim())
	}
	re, err := r.Params().(rule.ContentParams).Regexp()
	if err != nil || !re.MatchString("\tpanic(\"boom\")") || re.MatchString("// no panics here") {
		t.Errorf("Regexp = %v, %v", re, err)
	}
	for name, forbid := range map[string]string{"blank": "  ", "invalid": "("} {
		if _, err := rule.New(rule.Spec{
			ID:            "domain/no-panic",
			Type:          rule.TypeContent,
			Params:        rule.ContentParams{Forbid: forbid},
			Applicability: mustModuleApplicability(t, "domain"),
		}); err == nil {
			t.Errorf("%s forbid: expected error", name)
		}
	}
	repoWide, err := rule.New(rule.Spec{
		ID:            "repo/no-todo",
		Type:          rule.TypeContent,
		Params:        rule.ContentParams{Forbid: "TODO"},
		Applicability: mustRepoApplicability(t),
	})
	if err != nil {
		t.Fatalf("a content Rule ranges over the repository when on is omitted: %v", err)
	}
	if len(repoWide.ReferencedModules()) != 0 {
		t.Errorf("a repository-wide content Rule names no Module")
	}
}

func TestReferencedModules(t *testing.T) {
	allow, err := rule.NewAllowList("domain", "shared")
	if err != nil {
		t.Fatalf("NewAllowList: %v", err)
	}
	consumes, err := rule.New(rule.Spec{
		ID:            "application/imports",
		Type:          rule.TypeConsumes,
		Params:        rule.ConsumesParams{Internal: &allow, External: rule.ImportAllow},
		Applicability: mustModuleApplicability(t, "application", "domain"),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := names(consumes.ReferencedModules()); got != "application,domain,shared" {
		t.Errorf("consumes ReferencedModules = %s", got)
	}
	protected, err := rule.New(rule.Spec{
		ID:            "infra/only-composition",
		Type:          rule.TypeProtected,
		Params:        rule.ProtectedParams{Module: "infra", Allow: []rule.ModuleName{"composition", "infra"}},
		Applicability: mustRepoApplicability(t),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := names(protected.ReferencedModules()); got != "infra,composition" {
		t.Errorf("protected ReferencedModules = %s", got)
	}
	layers, err := rule.New(rule.Spec{
		ID:            "deps/inward",
		Type:          rule.TypeLayers,
		Params:        rule.LayersParams{Layers: []rule.ModuleName{"app", "domain"}},
		Applicability: mustRepoApplicability(t),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := names(layers.ReferencedModules()); got != "app,domain" {
		t.Errorf("layers ReferencedModules = %s", got)
	}
	acyclic, err := rule.New(rule.Spec{
		ID:            "deps/acyclic",
		Type:          rule.TypeAcyclic,
		Params:        rule.AcyclicParams{Modules: []rule.ModuleName{"a", "b"}},
		Applicability: mustRepoApplicability(t),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := names(acyclic.ReferencedModules()); got != "a,b" {
		t.Errorf("acyclic ReferencedModules = %s", got)
	}
}

func names(modules []rule.ModuleName) string {
	parts := make([]string, 0, len(modules))
	for _, m := range modules {
		parts = append(parts, m.String())
	}
	return strings.Join(parts, ",")
}

func TestRuleTestCompare(t *testing.T) {
	files := []rule.TestFile{{Path: "a/x.go", Content: "package a"}}
	// The vocabulary's duplicate-Diagnostics concern is settled at
	// construction: an expectation listed twice is unconstructible.
	if _, err := rule.NewTest("duplicates", "t:a/b", files, []rule.ExpectedFinding{
		{Path: "a/x.go", Message: "broken"},
		{Path: "a/x.go", Message: "broken"},
	}); err == nil {
		t.Errorf("duplicate expected findings must be unconstructible")
	}
	rt, err := rule.NewTest("compare", "t:a/b", files, []rule.ExpectedFinding{
		{Path: "a/x.go", Line: 3, Message: "broken"},
	})
	if err != nil {
		t.Fatalf("NewTest: %v", err)
	}
	cmp := rt.Compare([]rule.Finding{
		{Kind: rule.FindingViolation, Path: "a/y.go", Message: "surprise"},
	})
	if cmp.Clean() {
		t.Fatalf("mismatched result compared clean")
	}
	if len(cmp.Missing) != 1 || cmp.Missing[0].Message != "broken" {
		t.Errorf("missing = %v, want the unmet expectation", cmp.Missing)
	}
	if len(cmp.Unexpected) != 1 || cmp.Unexpected[0].Path != "a/y.go" {
		t.Errorf("unexpected = %v", cmp.Unexpected)
	}
}
