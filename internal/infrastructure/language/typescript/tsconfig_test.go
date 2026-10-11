package typescript

import (
	"testing"

	"github.com/wixregiga/arclint/internal/domain/conformance"
)

// importOf returns the classified import of spec in file.
func importOf(t *testing.T, facts map[string]conformance.LanguageFacts, file, spec string) conformance.Import {
	t.Helper()
	fa, ok := facts[file]
	if !ok {
		t.Fatalf("%s not analyzed", file)
	}
	for _, imp := range fa.Imports {
		if imp.Path == spec {
			return imp
		}
	}
	t.Fatalf("%s has no import %q: %+v", file, spec, fa.Imports)
	return conformance.Import{}
}

func expectTarget(t *testing.T, got conformance.Import, class conformance.ImportClass, file, dir string) {
	t.Helper()
	if got.Class != class || got.TargetFile != file || got.TargetDir != dir {
		t.Errorf("%s = {%s tf=%q td=%q}, want {%s tf=%q td=%q}", got.Path, got.Class, got.TargetFile, got.TargetDir, class, file, dir)
	}
}

// TestPathsAliasesResolveToFiles covers compilerOptions.paths the way
// TypeScript applies them: an exact pattern wins, else the wildcard with
// the longest prefix; substitutions are tried in order; targets resolve
// from the config when baseUrl is unset; extension and index probing
// match relative imports.
func TestPathsAliasesResolveToFiles(t *testing.T) {
	root, files := writeFiles(t, map[string]string{
		"package.json": `{"name": "web", "dependencies": {"effect": "4"}}`,
		"tsconfig.json": `{
  // comments and trailing commas are allowed in tsconfig
  "compilerOptions": {
    "paths": {
      "@/*": ["./src/*"],
      "@/ui/*": ["./vendor/ui/*", "./src/ui/*",],
      "config": ["./src/config/index.ts"],
      "effect": ["./missing/effect"], /* reaches no file */
    },
  },
}`,
		"src/main.ts": `import { a } from "@/domain/order";
import { b } from "@/ui/button";
import { c } from "config";
import { d } from "@/domain";
import { e } from "effect";
import { f } from "./domain/order";
import { g } from "@/missing/thing";
`,
		"src/domain/order.ts":    "export const a = 1;\n",
		"src/domain/index.ts":    "export const d = 1;\n",
		"src/ui/button.tsx":      "export const b = 1;\n",
		"src/config/index.ts":    "export const c = 1;\n",
		"vendor/ui/readme.md":    "not a module\n",
		"src/domain/order.d.txt": "noise\n",
	})
	facts := produce(t, root, files)
	expectTarget(t, importOf(t, facts, "src/main.ts", "@/domain/order"), conformance.ImportInternal, "src/domain/order.ts", "src/domain")
	expectTarget(t, importOf(t, facts, "src/main.ts", "@/ui/button"), conformance.ImportInternal, "src/ui/button.tsx", "src/ui")
	expectTarget(t, importOf(t, facts, "src/main.ts", "config"), conformance.ImportInternal, "src/config/index.ts", "src/config")
	expectTarget(t, importOf(t, facts, "src/main.ts", "@/domain"), conformance.ImportInternal, "src/domain/index.ts", "src/domain")
	expectTarget(t, importOf(t, facts, "src/main.ts", "effect"), conformance.ImportExternal, "", "")
	expectTarget(t, importOf(t, facts, "src/main.ts", "./domain/order"), conformance.ImportInternal, "src/domain/order.ts", "src/domain")
	expectTarget(t, importOf(t, facts, "src/main.ts", "@/missing/thing"), conformance.ImportUnknown, "", "")
}

// TestBaseURLAndExtendsChain covers baseUrl (bare specifiers resolve
// from it, and paths resolve from it when set), extends written without
// .json, the array form applied in order, and ${configDir} naming the
// extending config's directory.
func TestBaseURLAndExtendsChain(t *testing.T) {
	root, files := writeFiles(t, map[string]string{
		"tsconfig.base.json":    `{"compilerOptions": {"paths": {"~/*": ["${configDir}/src/*"]}}}`,
		"configs/strict.json":   `{"compilerOptions": {"baseUrl": "../wrong"}}`,
		"configs/base-url.json": `{"compilerOptions": {"baseUrl": "../apps/api/lib"}}`,
		"apps/api/tsconfig.json": `{"extends": ["../../tsconfig.base", "../../configs/strict.json", "../../configs/base-url.json"],
  "compilerOptions": {"paths": {"#db/*": ["db/*"]}}}`,
		"apps/web/tsconfig.json": `{"extends": "../../tsconfig.base.json"}`,
		"apps/api/src/main.ts": `import { a } from "helpers/clock";
import { b } from "#db/client";
import { c } from "~/routes";
`,
		"apps/api/lib/helpers/clock.ts": "export const a = 1;\n",
		"apps/api/lib/db/client.ts":     "export const b = 1;\n",
		"apps/web/src/main.ts":          `import { c } from "~/routes";` + "\n",
		"apps/web/src/routes.ts":        "export const c = 1;\n",
		"apps/api/src/routes.ts":        "export const c = 1;\n",
	})
	facts := produce(t, root, files)
	expectTarget(t, importOf(t, facts, "apps/api/src/main.ts", "helpers/clock"), conformance.ImportInternal, "apps/api/lib/helpers/clock.ts", "apps/api/lib/helpers")
	// TypeScript applies paths to "#" specifiers too (checked against
	// ts.resolveModuleName).
	expectTarget(t, importOf(t, facts, "apps/api/src/main.ts", "#db/client"), conformance.ImportInternal, "apps/api/lib/db/client.ts", "apps/api/lib/db")
	// apps/api's own paths override the base's, so "~/routes" is no alias there.
	expectTarget(t, importOf(t, facts, "apps/api/src/main.ts", "~/routes"), conformance.ImportUnknown, "", "")
	expectTarget(t, importOf(t, facts, "apps/web/src/main.ts", "~/routes"), conformance.ImportInternal, "apps/web/src/routes.ts", "apps/web/src")
}

// TestSolutionConfigDelegatesToReferences covers the layout Vite's
// templates write: a root tsconfig.json with "files": [] that only
// references tsconfig.app.json and tsconfig.node.json; the referenced
// config that includes the file governs it, and exclude removes files.
func TestSolutionConfigDelegatesToReferences(t *testing.T) {
	root, files := writeFiles(t, map[string]string{
		"tsconfig.json": `{"files": [], "references": [{"path": "./tsconfig.app.json"}, {"path": "./tsconfig.node.json"}]}`,
		"tsconfig.app.json": `{"compilerOptions": {"baseUrl": ".", "paths": {"@/*": ["./src/*"]}},
  "include": ["src"], "exclude": ["src/legacy"]}`,
		"tsconfig.node.json": `{"include": ["vite.config.ts"]}`,
		"src/main.ts":        `import { a } from "@/lib/a";` + "\n",
		"src/lib/a.ts":       "export const a = 1;\n",
		"src/legacy/old.ts":  `import { a } from "@/lib/a";` + "\n",
		"vite.config.ts":     `import { a } from "@/lib/a";` + "\n",
	})
	facts := produce(t, root, files)
	expectTarget(t, importOf(t, facts, "src/main.ts", "@/lib/a"), conformance.ImportInternal, "src/lib/a.ts", "src/lib")
	expectTarget(t, importOf(t, facts, "src/legacy/old.ts", "@/lib/a"), conformance.ImportUnknown, "", "")
	expectTarget(t, importOf(t, facts, "vite.config.ts", "@/lib/a"), conformance.ImportUnknown, "", "")
}

// TestNoConfigKeepsClassification proves a repository without tsconfig
// classifies exactly as before, and an invalid tsconfig is skipped.
func TestNoConfigKeepsClassification(t *testing.T) {
	root, files := writeFiles(t, map[string]string{
		"package.json":       `{"dependencies": {"effect": "4"}}`,
		"pkg/tsconfig.json":  `{ not json`,
		"src/main.ts":        `import { a } from "@/x";` + "\n" + `import { b } from "effect";` + "\n",
		"pkg/src/main.ts":    `import { a } from "@/x";` + "\n",
		"src/x.ts":           "export const a = 1;\n",
		"pkg/src/x/index.ts": "export const a = 1;\n",
	})
	facts := produce(t, root, files)
	expectTarget(t, importOf(t, facts, "src/main.ts", "@/x"), conformance.ImportUnknown, "", "")
	expectTarget(t, importOf(t, facts, "src/main.ts", "effect"), conformance.ImportExternal, "", "")
	expectTarget(t, importOf(t, facts, "pkg/src/main.ts", "@/x"), conformance.ImportUnknown, "", "")
}

// TestSegmentMatchKnowsOnlyStarAndQuestionMark pins TypeScript's
// wildcards: "*" and "?" match, every other character is itself.
func TestSegmentMatchKnowsOnlyStarAndQuestionMark(t *testing.T) {
	for _, tc := range []struct {
		pattern, name string
		want          bool
	}{
		{"*.ts", "order.ts", true},
		{"*.ts", "order.tsx", false},
		{"*", "", true},
		{"a*b*c", "aXXbYc", true},
		{"a*b*c", "aXXbY", false},
		{"?x.ts", "ax.ts", true},
		{"?x.ts", "x.ts", false},
		{"[ab].ts", "[ab].ts", true},
		{"[ab].ts", "a.ts", false},
		{"[id]", "[id]", true},
		{`a\b`, `a\b`, true},
		{"*.test.ts", "order.test.ts", true},
		{"é?", "éa", true},
	} {
		if got := segmentMatch(tc.pattern, tc.name); got != tc.want {
			t.Errorf("segmentMatch(%q, %q) = %v, want %v", tc.pattern, tc.name, got, tc.want)
		}
	}
}

func TestStripJSONC(t *testing.T) {
	in := "\ufeff{\n  // line\n  \"a\": \"keep // and /* this */\", /* block */\n  \"b\": [1, 2,],\n  \"c\": \"quote \\\" comma,]\",\n}\n"
	want := "{\n  \n  \"a\": \"keep // and /* this */\",  \n  \"b\": [1, 2],\n  \"c\": \"quote \\\" comma,]\"\n}\n"
	if got := string(stripJSONC([]byte(in))); got != want {
		t.Errorf("stripJSONC =\n%q\nwant\n%q", got, want)
	}
}
