package main

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// This probe exercises the public SDK through YAML loading, observation,
// subject selection, and the actual TypeScript sandbox.
const sdkScopeProbe = `
import { defineRule, s } from "arclint";
export default defineRule({
  type: "scope-probe",
  params: s.object({ marker: s.string() }),
  check(ctx, params) {
    const evidence = ["src/excluded.go", "src/retired/old.go", "other/hidden.go"];
    for (const path of evidence) {
      if (!ctx.imports(path).length || ctx.facts(path) === null || !ctx.zoneOf(path).length) {
        throw new Error("repository evidence hidden: " + path);
      }
      if (!ctx.read(path).includes("func Hidden")) throw new Error("content hidden: " + path);
    }
    for (const path of ["../outside.go", "/etc/passwd", "missing.go", "other/ignored.go"]) {
      if (ctx.imports(path).length || ctx.facts(path) !== null || ctx.zoneOf(path).length) {
        throw new Error("unobserved facts leaked: " + path);
      }
      let denied = false;
      try { ctx.read(path); } catch (_) { denied = true; }
      if (!denied) throw new Error("unobserved content leaked: " + path);
    }
    const scope = ctx.scope();
    if (JSON.stringify(scope.files) !== JSON.stringify(["src/kept.go"]) ||
        JSON.stringify(scope.excludedFiles) !== JSON.stringify(["src/excluded.go", "src/retired/old.go"]) ||
        scope.exclusions.length !== 1 || scope.exclusions[0].reason !== "adopted code") {
      throw new Error("resolved scope changed: " + JSON.stringify(scope));
    }
    const files = ctx.files();
    const path = scope.files[0];
    ctx.report({path, message: JSON.stringify({
      files, filtered: ctx.files("**/*.go"), content: ctx.read(path),
      imports: ctx.imports(path), facts: ctx.facts(path),
      zones: ctx.zones(), zoneOf: ctx.zoneOf(path), domain: ctx.domain(),
      marker: params.marker, rendered: ctx.caseTerm("Order Line", "snake_case")
    })});
  }
});
`

const sdkScopeRules = `runtime: [go]
scan:
  exclude: [other/ignored.go]
zones:
  src: src/**
  retired: src/retired/**
  other: other/**
rules:
  sdk-probe:
    uses: scope-probe
    with:
      marker: unchanged
    exclude:
      paths: [src/excluded.go, src/retired/**]
      reason: adopted code
`

const sdkScopeContent = "package src\nimport \"fmt\"\nfunc Kept() { fmt.Println(\"ok\") }\n"

func TestSDKScopePreservesSelectedPayloads(t *testing.T) {
	var previous any
	for _, tc := range []struct{ name, selection string }{
		{"zones", "    on: src\n    files: '**/*.go'\n"},
		{"repository", "    files: 'src/**/*.go'\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, ".arclint/extensions/probe.ts", sdkScopeProbe)
			write(t, root, "rules.arclint.yaml", sdkScopeRules+tc.selection)
			write(t, root, "src/kept.go", sdkScopeContent)
			for _, path := range []string{"src/excluded.go", "src/retired/old.go", "other/hidden.go"} {
				write(t, root, path, "package hidden\nimport \"os\"\nfunc Hidden() { os.Exit(1) }\n")
			}
			write(t, root, "src/note.txt", "repository evidence")
			write(t, root, "other/ignored.go", "package ignored\n")
			stdout, stderr, code := runBin(t, root, os.Environ(), "check", "--format", "json")
			if code != 1 {
				t.Fatalf("exit %d, want probe report\n%s\n%s", code, stdout, stderr)
			}
			var diagnostics []diagnosticDoc
			if err := json.Unmarshal([]byte(stdout), &diagnostics); err != nil {
				t.Fatal(err)
			}
			var payload string
			for _, d := range diagnostics {
				if d.Kind == "operational" {
					t.Fatalf("SDK probe failed: %s", d.Message)
				}
				if d.Kind == "violation" && d.RuleID == "sdk-probe" {
					if payload != "" || d.Path != "src/kept.go" {
						t.Fatalf("unexpected SDK report: %+v", d)
					}
					payload = d.Message
				}
			}
			assertSDKScopePayload(t, payload)
			var decoded any
			if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
				t.Fatal(err)
			}
			// The observed rules file has different bytes for equivalent authored selectors.
			delete(decoded.(map[string]any), "files")
			if previous != nil && !reflect.DeepEqual(decoded, previous) {
				t.Fatalf("equivalent scopes changed SDK payload\n%v\n%v", previous, decoded)
			}
			previous = decoded
		})
	}
}

func assertSDKScopePayload(t *testing.T, payload string) {
	t.Helper()
	var got struct {
		Content, Marker, Rendered string
		Files, Filtered           []struct {
			Path, Name, Stem, Ext, Dir string
			Size                       int
		}
		Imports []struct{ Path, Class string }
		Facts   struct {
			Path, Package string
			Decls         []struct{ Name string }
		}
		Zones  map[string][]string
		ZoneOf []string
		Domain struct{ Contexts, Relations []any }
	}
	if err := json.Unmarshal([]byte(payload), &got); err != nil {
		t.Fatalf("SDK payload: %v\n%s", err, payload)
	}
	if got.Content != sdkScopeContent || got.Marker != "unchanged" || got.Rendered != "order_line" {
		t.Fatalf("content/params/casing changed: %s", payload)
	}
	if len(got.Files) != 6 || len(got.Filtered) != 4 {
		t.Fatalf("file selection changed: %s", payload)
	}
	f := got.Files[3]
	if f.Path != "src/kept.go" || f.Name != "kept.go" || f.Stem != "kept" || f.Ext != ".go" || f.Dir != "src" || f.Size != len(sdkScopeContent) {
		t.Fatalf("file metadata changed: %s", payload)
	}
	if len(got.Imports) != 1 || got.Imports[0].Path != "fmt" || got.Imports[0].Class != "stdlib" || got.Facts.Path != f.Path || got.Facts.Package != "src" || len(got.Facts.Decls) != 1 || got.Facts.Decls[0].Name != "Kept" {
		t.Fatalf("language facts changed: %s", payload)
	}
	if len(got.ZoneOf) != 1 || got.ZoneOf[0] != "src" || len(got.Zones) != 3 || len(got.Zones["src"]) != 4 || len(got.Zones["retired"]) != 1 || len(got.Zones["other"]) != 1 {
		t.Fatalf("zone visibility changed: %s", payload)
	}
	if got.Domain.Contexts == nil || got.Domain.Relations == nil || len(got.Domain.Contexts) != 0 || len(got.Domain.Relations) != 0 {
		t.Fatalf("empty domain shape changed: %s", payload)
	}
}
