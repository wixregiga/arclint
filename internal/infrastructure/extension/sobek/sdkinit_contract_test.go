package sobekextension

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The editor contract must match the generated wire types and shared API.
// tools/gensdktypes separately regenerates the wire types from current Go code.
func TestPublishedSDKMatchesContract(t *testing.T) {
	root := t.TempDir()
	if _, err := SDKInit(root); err != nil {
		t.Fatal(err)
	}
	relative := filepath.Join(".arclint", "extensions", "arclint.d.ts")
	generated, err := os.ReadFile(filepath.Join(root, relative))
	if err != nil {
		t.Fatal(err)
	}
	for _, project := range []string{".", "testing/boxoffice"} {
		t.Run(project, func(t *testing.T) {
			published, err := os.ReadFile(filepath.Join("..", "..", "..", "..", project, relative))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(generated, published) {
				t.Fatalf("published SDK declarations in %s are stale; regenerate the SDK and run arclint sdk init in that project", project)
			}
		})
	}
}

// Generation proves declaration consistency; this test separately exercises
// the declared calls through the embedded SDK and the real JavaScript host.
// Adding a Ctx method requires adding a behavioral probe here as well.
func TestPublishedContextCallsMatchHost(t *testing.T) {
	file := FileInfo{Path: "src/order.ts", Name: "order.ts", Stem: "order", Ext: ".ts", Dir: "src", Size: 23}
	imports := []ImportInfo{{Path: "../dep/item", Line: 2, Class: "internal", TargetDir: "dep", TargetFile: "dep/item.ts", TargetZones: []string{"dep"}, TargetObserved: true}}
	facts := FactsInfo{
		Path: file.Path, Zones: []string{"src"}, ImportsAvailable: true, Imports: imports,
		Dependencies:          []DependencyInfo{{SourcePath: file.Path, TargetPath: "dep/item.ts", TargetKind: "file", Specifier: "../dep/item", Line: 2, Classification: "internal", SourceZones: []string{"src"}, TargetZones: []string{"dep"}, TargetObserved: true}},
		DeclarationsAvailable: true,
		Decls:                 []DeclInfo{{Kind: "func", Name: "Order", Exported: true, StartLine: 3, EndLine: 4, Params: []ParamInfo{{Name: "id", Type: "string", Optional: true}}, Results: []string{"string"}}},
	}
	domain := DomainInfo{Source: "domain.arclint.yaml", Project: "contract", Contexts: []DomainContextInfo{}, Relations: []DomainRelationInfo{}}
	report := ViolationInput{SubjectPath: file.Path, Path: "adapter/handler.ts", Line: 7, Message: "contract", FixHint: "use allowed importer"}
	reportJSON, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	probes := map[string]string{
		"files":    `ctx.files("src/**")`,
		"read":     `ctx.read("src/order.ts")`,
		"imports":  `ctx.imports("src/order.ts")`,
		"zones":    `ctx.zones()`,
		"facts":    `ctx.facts("src/order.ts")`,
		"zoneOf":   `ctx.zoneOf("src/order.ts")`,
		"domain":   `ctx.domain()`,
		"caseTerm": `ctx.caseTerm("Order Item", "snake_case")`,
		"report":   "ctx.report(" + string(reportJSON) + ")",
	}
	declaration := strings.SplitN(strings.SplitN(sdkAPIDecl, "export interface Ctx {", 2)[1], "\n}", 2)[0]
	var declared []string
	for _, match := range regexp.MustCompile(`(?m)^  (\w+)\(`).FindAllStringSubmatch(declaration, -1) {
		declared = append(declared, match[1])
	}
	var exercised, fields []string
	for name, call := range probes {
		exercised = append(exercised, name)
		fields = append(fields, name+": "+call)
	}
	sort.Strings(declared)
	sort.Strings(exercised)
	sort.Strings(fields)
	if !reflect.DeepEqual(declared, exercised) {
		t.Fatalf("declared Ctx calls %v differ from behavioral probes %v", declared, exercised)
	}
	names, err := json.Marshal(declared)
	if err != nil {
		t.Fatal(err)
	}
	source := fmt.Sprintf(`import { defineRule, s } from "arclint";
export default defineRule({type: "host-contract", description: "contract", capability: "structural",
  params: s.object({label: s.string().default("default").describe("label"),
    number: s.number(), integer: s.integer(), boolean: s.boolean(),
    choice: s.enum("one", "two"), items: s.array(s.string()), optional: s.string().optional()}),
  check(ctx, params) {
    if (JSON.stringify(Object.keys(ctx).sort()) !== '%s') throw new Error("host surface differs");
    if (params.label !== "default" || "optional" in params) throw new Error("schema defaults differ");
    const result = {%s};
    if (result.report !== undefined) throw new Error("report must return void");
    if (ctx.facts("missing") !== null) throw new Error("missing facts must be null");
    ctx.report({path: "src/order.ts", message: JSON.stringify(result)});
  }
});`, names, strings.Join(fields, ",\n"))
	registry, err := Load(t.TempDir(), []SuppliedSource{{Name: "host-contract.ts", Source: source}}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	rt := registry.Get("host-contract")
	if rt == nil || rt.SourcePath != "host-contract.ts" || rt.Description != "contract" || rt.Capability != "structural" {
		t.Fatalf("registration metadata: %+v", rt)
	}
	input := map[string]any{"number": 1.5, "integer": 2, "boolean": true, "choice": "one", "items": []string{"item"}}
	params, err := rt.ValidateParams(input)
	if err != nil {
		t.Fatal(err)
	}
	for field, invalid := range map[string]any{"number": "1.5", "integer": 1.5, "boolean": "true", "choice": "three", "items": []int{1}, "optional": false, "label": 42} {
		t.Run(field, func(t *testing.T) {
			bad := make(map[string]any, len(input)+1)
			for key, value := range input {
				bad[key] = value
			}
			bad[field] = invalid
			if _, err := rt.ValidateParams(bad); err == nil {
				t.Fatal("schema accepted invalid parameter")
			}
		})
	}
	checkPath := func(path string) {
		t.Helper()
		if path != file.Path {
			t.Errorf("host received path %q, want %q", path, file.Path)
		}
	}
	host := Host{
		Files: func(glob string) ([]FileInfo, error) {
			if glob != "src/**" {
				t.Errorf("host received glob %q", glob)
			}
			return []FileInfo{file}, nil
		},
		Read:    func(path string) (string, error) { checkPath(path); return "content", nil },
		Imports: func(path string) []ImportInfo { checkPath(path); return imports },
		Zones:   func() map[string][]string { return map[string][]string{"src": {file.Path}} },
		Facts: func(path string) *FactsInfo {
			if path == "missing" {
				return nil
			}
			checkPath(path)
			return &facts
		},
		ZoneOf: func(path string) []string { checkPath(path); return []string{"src"} },
		Domain: func() DomainInfo { return domain },
		CaseTerm: func(term, termCase string) (string, error) {
			if term != "Order Item" || termCase != "snake_case" {
				t.Errorf("caseTerm arguments: %q %q", term, termCase)
			}
			return "order_item", nil
		},
	}
	got, err := rt.Check(host, params)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != report || got[1].Path != file.Path {
		t.Fatalf("report input changed: %+v", got)
	}
	wantJSON, err := json.Marshal(map[string]any{
		"files": []FileInfo{file}, "read": "content", "imports": imports,
		"zones": map[string][]string{"src": {file.Path}}, "facts": facts,
		"zoneOf": []string{"src"}, "domain": domain, "caseTerm": "order_item",
	})
	if err != nil {
		t.Fatal(err)
	}
	var actual, expected any
	if err := json.Unmarshal([]byte(got[1].Message), &actual); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(wantJSON, &expected); err != nil {
		t.Fatal(err)
	}
	// The JS field mapper exposes optional fields even at their zero value;
	// Go's JSON encoder omits them. Both satisfy the published optional types.
	expectedFacts := expected.(map[string]any)["facts"].(map[string]any)
	expectedFacts["parseError"] = ""
	decl := expectedFacts["decls"].([]any)[0].(map[string]any)
	decl["params"].([]any)[0].(map[string]any)["variadic"] = false
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("host wire shapes differ:\ngot  %s\nwant %s", got[1].Message, wantJSON)
	}
}
