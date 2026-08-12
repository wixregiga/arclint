package ext_test

// M4 authoring metadata: defineRule descriptions, per-finding
// contract/blame overrides, and the targetFile field crossing the JS
// boundary.

import (
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/ext"
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
	reg, err := ext.LoadDir(root, ext.Options{})
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

func TestPerFindingContractBlameOverride(t *testing.T) {
	root := writeExtensions(t, map[string]string{"override.ts": `
import { defineRule } from "arclint";
export default defineRule({
  type: "override",
  contract: "provides",
  blame: "provider",
  check(ctx) {
    ctx.report({ path: "a.go", message: "overridden", contract: "consumes", blame: "consumer" });
    ctx.report({ path: "b.go", message: "type default" });
  },
});
`})
	reg, err := ext.LoadDir(root, ext.Options{})
	if err != nil {
		t.Fatal(err)
	}
	rt := reg.Get("override")
	params, err := rt.ValidateParams(nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := rt.Check(fakeHost(nil), params)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("violations: %+v", got)
	}
	if got[0].Contract != "consumes" || got[0].Blame != "consumer" {
		t.Errorf("override not carried: %+v", got[0])
	}
	if got[1].Contract != "" || got[1].Blame != "" {
		t.Errorf("absent override must stay empty for the engine to apply the type default: %+v", got[1])
	}
}

func TestInvalidContractOverrideRejected(t *testing.T) {
	root := writeExtensions(t, map[string]string{"bad.ts": `
import { defineRule } from "arclint";
export default defineRule({
  type: "bad",
  check(ctx) {
    ctx.report({ path: "a.go", message: "m", contract: "nonsense" });
  },
});
`})
	reg, err := ext.LoadDir(root, ext.Options{})
	if err != nil {
		t.Fatal(err)
	}
	rt := reg.Get("bad")
	params, err := rt.ValidateParams(nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rt.Check(fakeHost(nil), params); err == nil || !strings.Contains(err.Error(), "invalid contract") {
		t.Errorf("err = %v, want invalid contract", err)
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
	reg, err := ext.LoadDir(root, ext.Options{})
	if err != nil {
		t.Fatal(err)
	}
	rt := reg.Get("imports-view")
	params, err := rt.ValidateParams(nil)
	if err != nil {
		t.Fatal(err)
	}
	host := fakeHost(nil)
	host.Imports = func(path string) []ext.ImportInfo {
		return []ext.ImportInfo{{
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
