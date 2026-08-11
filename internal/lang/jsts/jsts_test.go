package jsts

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/wixregiga/arclint/internal/lang"
	"github.com/wixregiga/arclint/internal/tree"
)

// TestExtractForms covers exactly the static forms the research report
// (multi-language-rule-engines.md §4) commits to: import/export-from/
// require plus literal dynamic import.
func TestExtractForms(t *testing.T) {
	src := `import def from "m-default";
import * as ns from 'm-namespace';
import { a, b as c } from "m-named";
import "m-side-effect";
import type { T } from "m-type-only";
import {
  multi1,
  multi2,
} from "m-multiline";
export * from "m-reexport-star";
export * as grouped from "m-reexport-ns";
export { x } from "m-reexport-named";
export type { Y } from "m-reexport-type";
const cjs = require("m-cjs");
const dyn = await import("m-dynamic-literal");
`
	got := extract(src)
	want := []string{
		"m-default", "m-namespace", "m-named", "m-side-effect", "m-type-only",
		"m-multiline", "m-reexport-star", "m-reexport-ns", "m-reexport-named",
		"m-reexport-type", "m-cjs", "m-dynamic-literal",
	}
	var specs []string
	for _, ri := range got {
		specs = append(specs, ri.spec)
	}
	if !reflect.DeepEqual(specs, want) {
		t.Errorf("extracted %v\nwant %v", specs, want)
	}
	// Line anchors: first statement line 1, multiline statement anchors at
	// its specifier's line.
	if got[0].line != 1 {
		t.Errorf("line of first import = %d", got[0].line)
	}
	if got[5].line != 9 {
		t.Errorf("line of multiline specifier = %d, want 9", got[5].line)
	}
}

// TestDocumentedFalseNegatives asserts the false-negative classes stay
// false negatives: computed specifiers are not extractable at this tier,
// and that is the documented contract, not an accident.
func TestDocumentedFalseNegatives(t *testing.T) {
	src := `const name = "m-hidden";
const a = import(name);
const b = require(name);
const c = require(` + "`m-template`" + `);
const d = import(` + "`m-${name}`" + `);
const e = "import fake from 'm-in-string'";
// import commented from "m-comment";
/* import blocked from "m-block"; */
const f = ` + "`import tpl from 'm-in-template'`" + `;
`
	got := extract(src)
	if len(got) != 0 {
		t.Errorf("computed/quoted specifiers extracted: %+v", got)
	}
}

func TestExtractInsideTemplateInterpolation(t *testing.T) {
	src := "const x = `prefix ${require(\"m-interp\")} suffix`;\n"
	got := extract(src)
	if len(got) != 1 || got[0].spec != "m-interp" {
		t.Errorf("interpolation require: %+v", got)
	}
}

func TestRegexLiteralDoesNotBreakScanning(t *testing.T) {
	src := `const re = /["']import/;
import real from "m-after-regex";
`
	got := extract(src)
	if len(got) != 1 || got[0].spec != "m-after-regex" {
		t.Errorf("after regex literal: %+v", got)
	}
}

func writeTree(t *testing.T, files map[string]string) *tree.Tree {
	t.Helper()
	root := t.TempDir()
	for p, content := range files {
		abs := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	tr, err := tree.Walk(root, tree.Options{})
	if err != nil {
		t.Fatal(err)
	}
	return tr
}

func TestClassification(t *testing.T) {
	tr := writeTree(t, map[string]string{
		"package.json": `{"name": "app", "dependencies": {"lodash": "^4"}, "devDependencies": {"@types/node": "*"}}`,
		"src/main.ts": `import fs from "node:fs";
import path from "path";
import _ from "lodash";
import t from "@types/node/fs";
import util from "./util";
import deep from "../src/lib/index";
import missing from "./nowhere";
import ghost from "unknown-pkg";
`,
		"src/util.ts":      "export const u = 1;\n",
		"src/lib/index.ts": "export const l = 1;\n",
	})
	a := Analyze(tr)
	fa := a.Files["src/main.ts"]
	if fa == nil {
		t.Fatal("main.ts not analyzed")
	}
	type want struct {
		spec  string
		class lang.Class
		tf    string
		td    string
	}
	wants := []want{
		{"node:fs", lang.ClassStdlib, "", ""},
		{"path", lang.ClassStdlib, "", ""},
		{"lodash", lang.ClassExternal, "", ""},
		{"@types/node/fs", lang.ClassExternal, "", ""},
		{"./util", lang.ClassInternal, "src/util.ts", "src"},
		{"../src/lib/index", lang.ClassInternal, "src/lib/index.ts", "src/lib"},
		{"./nowhere", lang.ClassInternal, "", ""},
		{"unknown-pkg", lang.ClassUnknown, "", ""},
	}
	if len(fa.Imports) != len(wants) {
		t.Fatalf("imports: %+v", fa.Imports)
	}
	for i, w := range wants {
		got := fa.Imports[i]
		if got.Path != w.spec || got.Class != w.class || got.TargetFile != w.tf || got.TargetDir != w.td {
			t.Errorf("[%d] = {%s %s tf=%q td=%q}, want {%s %s tf=%q td=%q}",
				i, got.Path, got.Class, got.TargetFile, got.TargetDir, w.spec, w.class, w.tf, w.td)
		}
	}
}

func TestMonorepoWorkspaceNameResolvesInternal(t *testing.T) {
	tr := writeTree(t, map[string]string{
		"package.json":               `{"name": "root", "workspaces": ["packages/*"]}`,
		"packages/core/package.json": `{"name": "@app/core"}`,
		"packages/web/package.json":  `{"name": "@app/web", "dependencies": {"@app/core": "workspace:*", "react": "^18"}}`,
		"packages/web/index.ts":      `import { boot } from "@app/core";` + "\n" + `import React from "react";` + "\n",
		"packages/core/index.ts":     "export const boot = 1;\n",
	})
	a := Analyze(tr)
	fa := a.Files["packages/web/index.ts"]
	if fa == nil || len(fa.Imports) != 2 {
		t.Fatalf("imports: %+v", fa)
	}
	if fa.Imports[0].Class != lang.ClassInternal || fa.Imports[0].TargetDir != "packages/core" {
		t.Errorf("workspace name: %+v", fa.Imports[0])
	}
	if fa.Imports[1].Class != lang.ClassExternal {
		t.Errorf("react: %+v", fa.Imports[1])
	}
}

func TestNestedManifestNearestWins(t *testing.T) {
	tr := writeTree(t, map[string]string{
		"package.json":          `{"dependencies": {"root-only": "1"}}`,
		"apps/api/package.json": `{"dependencies": {"api-only": "1"}}`,
		"apps/api/server.js":    `const a = require("api-only");` + "\n" + `const r = require("root-only");` + "\n",
	})
	a := Analyze(tr)
	fa := a.Files["apps/api/server.js"]
	if fa.Imports[0].Class != lang.ClassExternal {
		t.Errorf("api-only: %+v", fa.Imports[0])
	}
	// root-only is not declared by the nearest manifest: unknown, matching
	// the nested go.mod ownership rule.
	if fa.Imports[1].Class != lang.ClassUnknown {
		t.Errorf("root-only: %+v", fa.Imports[1])
	}
}

func TestDtsSkipped(t *testing.T) {
	tr := writeTree(t, map[string]string{
		"types.d.ts": `import "phantom";`,
		"real.ts":    `import "node:fs";`,
	})
	a := Analyze(tr)
	if a.Files["types.d.ts"] != nil {
		t.Error(".d.ts analyzed")
	}
	if a.Files["real.ts"] == nil {
		t.Error("real.ts not analyzed")
	}
}

func TestStdlibTableSanity(t *testing.T) {
	for _, m := range []string{"fs", "node:fs", "path", "fs/promises", "child_process"} {
		if !IsStdlib(m) {
			t.Errorf("%s missing from Node builtin table", m)
		}
	}
	for _, m := range []string{"lodash", "react", "@scope/x"} {
		if IsStdlib(m) {
			t.Errorf("%s wrongly stdlib", m)
		}
	}
}
