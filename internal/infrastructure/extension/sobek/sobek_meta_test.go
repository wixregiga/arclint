package sobekextension_test

// Authoring metadata: defineRule descriptions and the targetFile field
// crossing the JS boundary.

import (
	"testing"

	sobekextension "github.com/wixregiga/arclint/internal/infrastructure/extension/sobek"
)

func TestDescriptionPassthrough(t *testing.T) {
	root := writeExtensions(t, map[string]string{"described.ts": `
import { defineRule } from "arclint";
export default defineRule({
  type: "described",
  description: "Counts widgets per module.",
  check(ctx) {},
});
`})
	reg, err := sobekextension.LoadDir(root, sobekextension.Options{})
	if err != nil {
		t.Fatal(err)
	}
	rt := reg.Get("described")
	if rt.Description != "Counts widgets per module." {
		t.Errorf("Description = %q", rt.Description)
	}
	if rt.Describe() != rt.Description {
		t.Errorf("Describe() = %q, want the declared description", rt.Describe())
	}
}

func TestImportTargetFileCrossesBoundary(t *testing.T) {
	root := writeExtensions(t, map[string]string{"imports.ts": `
import { defineRule } from "arclint";
export default defineRule({
  type: "imports-view",
  check(ctx) {
    for (const imp of ctx.imports("src/a.ts")) {
      ctx.report({ path: "src/a.ts", line: imp.line,
        message: imp.class + " " + imp.path + " -> " + imp.targetFile });
    }
  },
});
`})
	reg, err := sobekextension.LoadDir(root, sobekextension.Options{})
	if err != nil {
		t.Fatal(err)
	}
	rt := reg.Get("imports-view")
	params, err := rt.ValidateParams(nil)
	if err != nil {
		t.Fatal(err)
	}
	host := fakeHost(nil)
	host.Imports = func(path string) []sobekextension.ImportInfo {
		return []sobekextension.ImportInfo{{
			Path: "./b", Line: 3, Class: "internal",
			TargetDir: "src", TargetFile: "src/b.ts",
		}}
	}
	got, err := rt.Check(host, params)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Message != "internal ./b -> src/b.ts" || got[0].Line != 3 {
		t.Fatalf("violations: %+v", got)
	}
}

func TestFactsCrossTheBoundary(t *testing.T) {
	root := writeExtensions(t, map[string]string{"facts.ts": `
import { defineRule } from "arclint";
export default defineRule({
  type: "facts-view",
  check(ctx) {
    const facts = ctx.facts("a/a.go");
    if (facts === null) {
      ctx.report({ path: "a/a.go", message: "facts unavailable" });
      return;
    }
    for (const d of facts.decls) {
      if (d.kind === "method" && d.exported) {
        ctx.report({ path: facts.path, line: d.startLine,
          message: d.owner + "." + d.name + "/" + (d.params ?? []).length });
      }
    }
    if (ctx.facts("b/b.ts") !== null) {
      ctx.report({ path: "a/a.go", message: "ts facts should be null" });
    }
  },
});
`})
	reg, err := sobekextension.LoadDir(root, sobekextension.Options{})
	if err != nil {
		t.Fatal(err)
	}
	rt := reg.Get("facts-view")
	params, err := rt.ValidateParams(nil)
	if err != nil {
		t.Fatal(err)
	}
	host := fakeHost(nil)
	host.Facts = func(path string) *sobekextension.FactsInfo {
		if path != "a/a.go" {
			return nil
		}
		return &sobekextension.FactsInfo{
			Path: "a/a.go", Package: "a",
			Decls: []sobekextension.DeclInfo{{
				Kind: "method", Name: "Save", Owner: "Store", Exported: true,
				StartLine: 12, EndLine: 14,
				Params: []sobekextension.ParamInfo{{Name: "w", Type: "io.Writer"}},
			}},
		}
	}
	got, err := rt.Check(host, params)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Message != "Store.Save/1" || got[0].Line != 12 {
		t.Fatalf("violations: %+v", got)
	}
}

func TestFactsNullWithoutCapability(t *testing.T) {
	root := writeExtensions(t, map[string]string{"nofacts.ts": `
import { defineRule } from "arclint";
export default defineRule({
  type: "no-facts",
  check(ctx) {
    if (ctx.facts("x.go") === null) {
      ctx.report({ path: "x.go", message: "null as documented" });
    }
  },
});
`})
	reg, err := sobekextension.LoadDir(root, sobekextension.Options{})
	if err != nil {
		t.Fatal(err)
	}
	rt := reg.Get("no-facts")
	params, err := rt.ValidateParams(nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := rt.Check(fakeHost(nil), params) // fakeHost lends no Facts capability
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Message != "null as documented" {
		t.Fatalf("violations: %+v", got)
	}
}
