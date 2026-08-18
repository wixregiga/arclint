package typescript

import (
	"reflect"
	"testing"
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
