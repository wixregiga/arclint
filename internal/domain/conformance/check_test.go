package conformance_test

import (
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
)

func mustGlob(t *testing.T, pattern string) rule.Glob {
	t.Helper()
	g, err := rule.NewGlob(pattern)
	if err != nil {
		t.Fatalf("NewGlob(%q): %v", pattern, err)
	}
	return g
}

func mustModule(t *testing.T, name, glob string) rule.Module {
	t.Helper()
	m, err := rule.NewModule(rule.ModuleName(name), "", []rule.Glob{mustGlob(t, glob)})
	if err != nil {
		t.Fatalf("NewModule(%q): %v", name, err)
	}
	return m
}

func mustRule(t *testing.T, spec rule.Spec) rule.Rule {
	t.Helper()
	r, err := rule.New(spec)
	if err != nil {
		t.Fatalf("rule.New(%s): %v", spec.ID, err)
	}
	return r
}

func moduleScope(t *testing.T, names ...rule.ModuleName) rule.Applicability {
	t.Helper()
	a, err := rule.ModuleApplicability(names)
	if err != nil {
		t.Fatalf("ModuleApplicability(%v): %v", names, err)
	}
	return a
}

func repoScope(t *testing.T) rule.Applicability {
	t.Helper()
	a, err := rule.RepositoryApplicability()
	if err != nil {
		t.Fatalf("RepositoryApplicability: %v", err)
	}
	return a
}

// scenario: Modules alpha and beta import each other; alpha restricts
// its imports; one file breaks naming; one file is forbidden by
// structure; one file fails analysis; one import is unknown; expr
// enforcement is not implemented in this build.
func scenarioRequest(t *testing.T, policy rule.UnknownImportPolicy) conformance.Request {
	t.Helper()
	modules := []rule.Module{
		mustModule(t, "alpha", "alpha/**"),
		mustModule(t, "beta", "beta/**"),
	}
	files := []conformance.ObservedFile{
		{Path: "alpha/broken.go"},
		{Path: "alpha/service.go"},
		{Path: "beta/BadName.go"},
		{Path: "beta/data.yaml"},
		{Path: "beta/root.go"},
	}
	facts := map[string]conformance.LanguageFacts{
		"alpha/service.go": {Language: rule.LanguageGo, ImportsAvailable: true, Imports: []conformance.Import{
			{Path: "fmt", Line: 3, Class: conformance.ImportStdlib},
			{Path: "example.com/mod/beta", Line: 4, Class: conformance.ImportInternal, TargetDir: "beta"},
			{Path: "github.com/x/y", Line: 5, Class: conformance.ImportExternal},
			{Path: "mystery", Line: 6, Class: conformance.ImportUnknown},
		}},
		"alpha/broken.go": {Language: rule.LanguageGo, ImportsAvailable: true, ParseFailure: "parse: bad syntax"},
		"beta/BadName.go": {Language: rule.LanguageGo, ImportsAvailable: true},
		"beta/root.go": {Language: rule.LanguageGo, ImportsAvailable: true, Imports: []conformance.Import{
			{Path: "example.com/mod/alpha", Line: 3, Class: conformance.ImportInternal, TargetDir: "alpha"},
		}},
	}
	obs, err := conformance.NewObservations(files, facts)
	if err != nil {
		t.Fatalf("NewObservations: %v", err)
	}

	emptyAllow, err := rule.NewAllowList()
	if err != nil {
		t.Fatalf("NewAllowList: %v", err)
	}
	snake, err := rule.NewCaseSpec("snake_case")
	if err != nil {
		t.Fatalf("NewCaseSpec: %v", err)
	}
	suppression, err := rule.NewSuppression([]rule.Glob{mustGlob(t, "alpha/**")}, "adopted legacy import")
	if err != nil {
		t.Fatalf("NewSuppression: %v", err)
	}
	disablement, err := rule.NewDisablement("migration in progress")
	if err != nil {
		t.Fatalf("NewDisablement: %v", err)
	}

	rules := []rule.Rule{
		mustRule(t, rule.Spec{
			ID:            "t:alpha/imports",
			Type:          rule.TypeConsumes,
			Params:        rule.ConsumesParams{Internal: &emptyAllow},
			Applicability: moduleScope(t, "alpha"),
		}),
		mustRule(t, rule.Spec{
			ID:   "t:beta/shape",
			Type: rule.TypeStructure,
			Params: rule.StructureParams{
				Require: []rule.Glob{mustGlob(t, "beta/root.go")},
				Forbid:  []rule.Glob{mustGlob(t, "**/*.yaml")},
			},
			Applicability: moduleScope(t, "beta"),
		}),
		mustRule(t, rule.Spec{
			ID:            "t:src/snake",
			Type:          rule.TypeNaming,
			Params:        rule.NamingParams{Case: snake},
			Applicability: moduleScope(t, "alpha", "beta"),
		}),
		mustRule(t, rule.Spec{
			ID:            "t:deps/layers",
			Type:          rule.TypeLayers,
			Params:        rule.LayersParams{Layers: []rule.ModuleName{"alpha", "beta"}},
			Applicability: repoScope(t),
		}),
		mustRule(t, rule.Spec{
			ID:            "t:deps/protected-beta",
			Type:          rule.TypeProtected,
			Params:        rule.ProtectedParams{Module: "beta"},
			Applicability: repoScope(t),
		}).Suppress(suppression),
		mustRule(t, rule.Spec{
			ID:            "t:deps/acyclic",
			Type:          rule.TypeAcyclic,
			Params:        rule.AcyclicParams{},
			Applicability: repoScope(t),
		}),
		mustRule(t, rule.Spec{
			ID:            "t:src/disabled",
			Type:          rule.TypeNaming,
			Params:        rule.NamingParams{Case: snake},
			Applicability: moduleScope(t, "alpha"),
		}).Disable(disablement),
	}
	return conformance.Request{
		Rules:          rules,
		Modules:        modules,
		Observations:   obs,
		UnknownImports: policy,
	}
}

func violationKeys(vs []conformance.Violation) []string {
	out := make([]string, 0, len(vs))
	for _, v := range vs {
		out = append(out, v.Rule().Qualified()+"|"+v.Path())
	}
	return out
}

func TestConformanceCheckScenario(t *testing.T) {
	a, err := conformance.Run(scenarioRequest(t, rule.UnknownImportsError))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	wantActive := []string{
		"t:alpha/imports|alpha/service.go",
		"t:beta/shape|beta/data.yaml",
		"t:deps/acyclic|alpha/service.go",
		"t:deps/acyclic|beta/root.go",
		"t:deps/layers|beta/root.go",
		"t:src/snake|beta/BadName.go",
	}
	active := sortedCopy(violationKeys(a.ActiveViolations()))
	if strings.Join(active, "\n") != strings.Join(wantActive, "\n") {
		t.Errorf("active violations:\n%s\nwant:\n%s",
			strings.Join(active, "\n"), strings.Join(wantActive, "\n"))
	}

	suppressed := a.SuppressedViolations()
	if len(suppressed) != 1 || suppressed[0].Rule().Qualified() != "t:deps/protected-beta" {
		t.Fatalf("suppressed = %v, want the protected finding", violationKeys(suppressed))
	}
	if suppressed[0].SuppressionReason() != "adopted legacy import" {
		t.Errorf("suppression reason = %q", suppressed[0].SuppressionReason())
	}
	if suppressed[0].FailsGate() {
		t.Errorf("a suppressed violation must not gate")
	}

	if !a.HasErrors() {
		t.Errorf("HasErrors = false with active error violations")
	}

	outcomes := map[conformance.Outcome]int{}
	for _, e := range a.Evaluations() {
		outcomes[e.Outcome()]++
	}
	if outcomes[conformance.OutcomeFailed] != 1 {
		t.Errorf("failed evaluations = %d, want 1 (unparseable file)", outcomes[conformance.OutcomeFailed])
	}

	for _, id := range a.AppliedRules() {
		if id == "t:src/disabled" {
			t.Errorf("disabled rule listed as applied")
		}
	}
	var sawDisabled, sawUnknownImport, sawParseFailure bool
	for _, d := range a.Diagnostics() {
		switch {
		case d.Kind() == conformance.DiagnosticCoverage && strings.Contains(d.Message(), "t:src/disabled"):
			sawDisabled = true
		case d.Kind() == conformance.DiagnosticOperational && strings.Contains(d.Message(), `import "mystery"`):
			sawUnknownImport = true
			if d.Severity() != rule.SeverityError {
				t.Errorf("unknown import severity = %q under error policy", d.Severity())
			}
		case d.Kind() == conformance.DiagnosticOperational && strings.Contains(d.Message(), "analysis failed"):
			sawParseFailure = true
		}
	}
	for name, saw := range map[string]bool{
		"disabled coverage": sawDisabled,
		"unknown import":    sawUnknownImport,
		"parse failure":     sawParseFailure,
	} {
		if !saw {
			t.Errorf("missing diagnostic: %s", name)
		}
	}
}

func TestConformanceCheckIsDeterministic(t *testing.T) {
	first, err := conformance.Run(scenarioRequest(t, rule.UnknownImportsWarn))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	second, err := conformance.Run(scenarioRequest(t, rule.UnknownImportsWarn))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	render := func(a conformance.Assessment) string {
		var b strings.Builder
		for _, d := range a.Diagnostics() {
			b.WriteString(d.RuleID())
			b.WriteString("|")
			b.WriteString(d.Path())
			b.WriteString("|")
			b.WriteString(d.Message())
			b.WriteString("\n")
		}
		return b.String()
	}
	if render(first) != render(second) {
		t.Errorf("two identical checks rendered differently")
	}
}

func TestFingerprintIsLineIndependent(t *testing.T) {
	subject, err := rule.FileSubject("alpha/service.go")
	if err != nil {
		t.Fatalf("FileSubject: %v", err)
	}
	id, err := rule.NewID("t:alpha/imports")
	if err != nil {
		t.Fatalf("NewID: %v", err)
	}
	build := func(line int) conformance.Violation {
		v, err := conformance.NewViolation(conformance.ViolationSpec{
			Rule: id, Subject: subject, Outcome: conformance.OutcomeViolates,
			Severity: rule.SeverityError, Assurance: rule.AssuranceExact,
			Evidence: "static import classification", Message: "same finding", Line: line,
		})
		if err != nil {
			t.Fatalf("NewViolation: %v", err)
		}
		return v
	}
	if build(4).Fingerprint() != build(99).Fingerprint() {
		t.Errorf("fingerprint changed when the finding moved lines")
	}
}

func TestRunRejectsUndeclaredModules(t *testing.T) {
	req := scenarioRequest(t, rule.UnknownImportsWarn)
	req.Modules = req.Modules[:1] // drop beta while rules still reference it
	if _, err := conformance.Run(req); err == nil {
		t.Errorf("expected an error for rules referencing undeclared modules")
	}
}

func sortedCopy(in []string) []string {
	out := append([]string(nil), in...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
