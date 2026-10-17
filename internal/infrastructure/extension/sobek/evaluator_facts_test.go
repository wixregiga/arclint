package sobekextension_test

import (
	"testing"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
	sobekextension "github.com/wixregiga/arclint/internal/infrastructure/extension/sobek"
)

func extensionFacts(t *testing.T, paths []string, obs conformance.Observations) conformance.Facts {
	t.Helper()
	var globs []rule.Glob
	for _, path := range paths {
		glob, err := rule.NewGlob(path)
		if err != nil {
			t.Fatal(err)
		}
		globs = append(globs, glob)
	}
	scope, err := rule.RepositoryScope(globs...)
	if err != nil {
		t.Fatal(err)
	}
	r, err := rule.New(rule.Spec{
		ID: "test/facts", Type: rule.TypeExtension,
		Params: rule.ExtensionParams{Uses: "probe"}, Scope: scope,
	})
	if err != nil {
		t.Fatal(err)
	}
	facts, err := conformance.NewFacts(r, nil, obs)
	if err != nil {
		t.Fatal(err)
	}
	return facts
}

// The source files and content capability deliberately do not exist. Facts
// must cross from the supplied observations without reading or parsing again.
func TestEvaluatorFactsUseSuppliedObservations(t *testing.T) {
	root := writeExtensions(t, map[string]string{"facts.ts": `
import { defineRule } from "arclint";
export default defineRule({
  type: "facts-probe",
  check(ctx) {
    function assert(ok, message) { if (!ok) throw new Error(message); }
    const empty = ctx.facts("empty.go");
    assert(empty.importsAvailable && empty.declarationsAvailable, "empty availability");
    assert(Array.isArray(empty.imports) && empty.imports.length === 0, "empty imports");
    assert(Array.isArray(empty.zones) && empty.zones.length === 0, "empty zones");
    const only = ctx.facts("imports.go");
    assert(only.importsAvailable && !only.declarationsAvailable, "imports without declarations");
    assert(only.decls.length === 0 && only.imports.length === 5, "supplied imports");
    assert(only.imports.map(i => i.class).join() === "stdlib,internal,external,unknown,cgo", "classes");
    const imp = only.imports[1];
    assert(imp.path === "example.test/pkg" && imp.line === 7, "specifier and source line");
    assert(imp.targetDir === "pkg" && imp.targetFile === "", "package resolution");
    assert(Array.isArray(imp.targetZones) && imp.targetZones.length === 0, "no invented zones");
    assert(imp.targetObserved === false, "unobserved package target");
    assert(only.dependencies.find(d => d.specifier === "example.test/pkg").targetObserved === false, "same dependency observation status");
    assert(JSON.stringify(only.imports) === JSON.stringify(ctx.imports("imports.go")), "same imports");
    for (const path of ["unsupported.go", "failed.go"]) {
      const facts = ctx.facts(path);
      assert(!facts.importsAvailable && facts.imports.length === 0, path + " imports unavailable");
      assert(!facts.declarationsAvailable && facts.decls.length === 0, path + " declarations unavailable");
    }
    assert(ctx.facts("failed.go").parseError === "supplied parse failure", "parse failure retained");
    assert(ctx.facts("missing.txt").importsAvailable === false, "missing observations");
    assert(ctx.facts("outside.go") === null, "outside Scope");
    // Mutating a response must not change the underlying observations.
    imp.path = "changed";
    only.imports.push(imp);
    const again = ctx.facts("imports.go");
    assert(again.imports.length === 5 && again.imports[1].path === "example.test/pkg", "immutable observations");
    ctx.report({path: "imports.go", message: "supplied facts preserved"});
  }
});
`})
	imports := []conformance.Import{
		{Path: "fmt", Line: 2, Class: conformance.ImportStdlib},
		{Path: "example.test/pkg", Line: 7, Class: conformance.ImportInternal, TargetDir: "pkg"},
		{Path: "third-party", Line: 8, Class: conformance.ImportExternal},
		{Path: "unresolved", Line: 9, Class: conformance.ImportUnknown},
		{Path: "C", Line: 10, Class: conformance.ImportCgo},
	}
	paths := []string{"empty.go", "imports.go", "unsupported.go", "failed.go", "missing.txt"}
	files := []conformance.ObservedFile{{Path: "outside.go"}}
	for _, path := range paths {
		files = append(files, conformance.ObservedFile{Path: path})
	}
	obs, err := conformance.NewObservations(files, map[string]conformance.LanguageFacts{
		"empty.go":       {Language: rule.LanguageGo, ImportsAvailable: true, DeclarationsAvailable: true},
		"imports.go":     {Language: rule.LanguageGo, ImportsAvailable: true, Imports: imports},
		"unsupported.go": {Language: rule.LanguageGo},
		"failed.go": {
			Language: rule.LanguageGo, ImportsAvailable: true, DeclarationsAvailable: true,
			ParseFailure: "supplied parse failure", Imports: imports,
			Declarations: []conformance.Declaration{{Kind: "func", Name: "Partial"}},
		},
		"outside.go": {Language: rule.LanguageGo, ImportsAvailable: true, Imports: imports},
	})
	if err != nil {
		t.Fatal(err)
	}
	evaluator, err := sobekextension.NewEvaluator(root)
	if err != nil {
		t.Fatal(err)
	}
	findings, err := evaluate(evaluator, "facts-probe", nil, extensionFacts(t, paths, obs), vocab.UbiquitousLanguage{})
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || findings[0].Message != "supplied facts preserved" {
		t.Fatalf("findings = %+v", findings)
	}
}

// An extension sees the calls, their resolution, and the types fields
// hold exactly as the built-in rules see them, through ctx.facts.
func TestEvaluatorFactsCarryResolvedCallsAndFieldTypes(t *testing.T) {
	root := writeExtensions(t, map[string]string{"resolved.ts": `
import { defineRule } from "arclint";
export default defineRule({
  type: "resolved-probe",
  check(ctx) {
    function assert(ok, message) { if (!ok) throw new Error(message); }
    const f = ctx.facts("order.go");
    assert(f.callsAvailable && f.resolvedCallsAvailable && f.fieldTypesAvailable, "availability");
    const save = f.calls.find(c => c.callee === "Save");
    assert(save.line === 12 && save.enclosing === "Place" && save.enclosingOwner === "Order", "call shape");
    assert(save.target.resolution === "declared" && save.target.file === "repo.go" && save.target.owner === "Repository" && save.target.name === "Save", "target");
    assert(save.receiver.resolution === "declared" && save.receiver.name === "Repository", "receiver");
    const now = f.calls.find(c => c.callee === "Now");
    assert(now.target.resolution === "external" && now.target.package === "time" && now.receiver == null, "external function");
    const clock = f.calls.find(c => c.callee === "clock");
    assert(clock.target.resolution === "undetermined" && clock.target.reason === "clock is a function value", "undetermined with reason");
    const repo = f.decls.find(d => d.kind === "field" && d.name === "repo");
    assert(repo.holds.length === 1 && repo.holds[0].file === "repo.go" && repo.holds[0].name === "Repository", "field holds");
    assert(repo.type.resolution === "declared" && repo.type.name === "Repository", "field type");
    const plain = ctx.facts("plain.go");
    assert(plain.callsAvailable && !plain.resolvedCallsAvailable && plain.calls[0].target == null, "unresolved calls carry no target");
    ctx.report({path: "order.go", message: "resolved facts seen"});
  }
});
`})
	declared := func(file, owner, name string) conformance.Reference {
		return conformance.Reference{Resolution: conformance.ResolutionDeclared, File: file, Owner: owner, Name: name}
	}
	files := []conformance.ObservedFile{{Path: "order.go"}, {Path: "repo.go"}, {Path: "plain.go"}}
	obs, err := conformance.NewObservations(files, map[string]conformance.LanguageFacts{
		"order.go": {
			Language: rule.LanguageGo, ImportsAvailable: true, DeclarationsAvailable: true, CallsAvailable: true,
			ResolvedCallsAvailable: true, FieldTypesAvailable: true,
			Declarations: []conformance.Declaration{
				{Kind: "struct", Name: "Order"},
				{Kind: "field", Name: "repo", Owner: "Order", Holds: []conformance.Reference{declared("repo.go", "", "Repository")}, Type: declared("repo.go", "", "Repository")},
			},
			Calls: []conformance.Call{
				{Callee: "Save", Line: 12, Enclosing: "Place", EnclosingOwner: "Order", Target: declared("repo.go", "Repository", "Save"), Receiver: declared("repo.go", "", "Repository")},
				{Callee: "Now", Line: 13, Enclosing: "Place", EnclosingOwner: "Order", Target: conformance.Reference{Resolution: conformance.ResolutionExternal, Package: "time", Name: "Now"}},
				{Callee: "clock", Line: 14, Enclosing: "Place", EnclosingOwner: "Order", Target: conformance.Reference{Resolution: conformance.ResolutionUndetermined, Reason: "clock is a function value"}},
			},
		},
		"repo.go": {
			Language: rule.LanguageGo, ImportsAvailable: true, DeclarationsAvailable: true, CallsAvailable: true,
			ResolvedCallsAvailable: true, FieldTypesAvailable: true,
			Declarations: []conformance.Declaration{{Kind: "interface", Name: "Repository"}, {Kind: "method", Name: "Save", Owner: "Repository"}},
		},
		"plain.go": {
			Language: rule.LanguageGo, ImportsAvailable: true, DeclarationsAvailable: true, CallsAvailable: true,
			Calls: []conformance.Call{{Callee: "run", Line: 3, Enclosing: "main"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	evaluator, err := sobekextension.NewEvaluator(root)
	if err != nil {
		t.Fatal(err)
	}
	findings, err := evaluate(evaluator, "resolved-probe", nil, extensionFacts(t, []string{"order.go", "repo.go", "plain.go"}, obs), vocab.UbiquitousLanguage{})
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || findings[0].Message != "resolved facts seen" {
		t.Fatalf("findings = %+v", findings)
	}
}

func TestEvaluatorCannotMutateKnowledgeForAnotherInvocation(t *testing.T) {
	root := writeExtensions(t, map[string]string{"mutate.ts": `
import { defineRule } from "arclint";
export default defineRule({type: "mutate", check(ctx) {
  const aliases = ctx.domain().contexts[0].valueObjects[0].aliases;
  if (aliases[0] !== "original") throw new Error("another invocation mutated knowledge");
  aliases[0] = "changed";
}});
`})
	evaluator, err := sobekextension.NewEvaluator(root)
	if err != nil {
		t.Fatal(err)
	}
	knowledge := vocab.UbiquitousLanguage{Contexts: []vocab.BoundedContext{{
		Name: "test", ValueObjects: []vocab.ValueObject{{Name: "Token", Aliases: []string{"original"}}},
	}}}
	for range 2 {
		if _, err := evaluate(evaluator, "mutate", nil, conformance.Facts{}, knowledge); err != nil {
			t.Fatal(err)
		}
	}
	if knowledge.Contexts[0].ValueObjects[0].Aliases[0] != "original" {
		t.Fatal("extension mutated project knowledge")
	}
}
