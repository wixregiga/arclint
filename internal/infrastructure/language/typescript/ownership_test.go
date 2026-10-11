package typescript

import (
	"testing"

	"github.com/wixregiga/arclint/internal/domain/conformance"
)

// ownersIn returns the owning configuration of every analyzed file in
// the layout, "" for none.
func ownersIn(t *testing.T, files map[string]string) map[string]string {
	t.Helper()
	root, observed := writeFiles(t, files)
	facts := map[string]conformance.LanguageFacts{}
	for _, f := range observed {
		if analyzable(f.Path) {
			facts[f.Path] = analyzeFile(nil, root, f.Path, false, false)
		}
	}
	r := newResolver(root, observed)
	r.classifyImports(facts)
	out := map[string]string{}
	for file, p := range r.owners {
		out[file] = ""
		if p != nil {
			out[file] = p.file
		}
	}
	return out
}

// TestOwnershipMatchesTSServer pins the configuration TypeScript's
// language service picks for each file. Every expected owner is
// tsserver 5.9.3's answer to projectInfo for the file opened in the
// same layout; "" is its inferred project.
func TestOwnershipMatchesTSServer(t *testing.T) {
	for _, tc := range []struct {
		layout string
		files  map[string]string
		owners map[string]string
	}{
		{
			layout: "the TypeScript 5.7 release-notes layout: src/tsconfig.json excludes tests, the root solution references src/tsconfig.test.json",
			files: map[string]string{
				"src/foo.test.ts":        `export const b = 1;`,
				"src/foo.ts":             `export const a = 1;`,
				"src/tsconfig.json":      `{"compilerOptions": {"outDir": "../dist", "composite": true}, "exclude": ["**/*.test.ts"]}`,
				"src/tsconfig.test.json": `{"compilerOptions": {"outDir": "../dist/test", "composite": true}, "include": ["**/*.test.ts"], "references": [{"path": "./tsconfig.json"}]}`,
				"tsconfig.json":          `{"files": [], "references": [{"path": "./src/tsconfig.json"}, {"path": "./src/tsconfig.test.json"}]}`,
			},
			owners: map[string]string{
				"src/foo.test.ts": "src/tsconfig.test.json",
				"src/foo.ts":      "src/tsconfig.json",
			},
		},
		{
			layout: "a reference to a configuration with any file name",
			files: map[string]string{
				"configs/frontend.json": `{"compilerOptions": {"composite": true}, "include": ["../src"]}`,
				"src/main.ts":           `export const a = 1;`,
				"tsconfig.json":         `{"files": [], "references": [{"path": "./configs/frontend.json"}]}`,
			},
			owners: map[string]string{
				"src/main.ts": "configs/frontend.json",
			},
		},
		{
			layout: "a solution that references a solution",
			files: map[string]string{
				"src/a.ts":           `export const a = 1;`,
				"tsconfig.json":      `{"files": [], "references": [{"path": "./tsconfig.mid.json"}]}`,
				"tsconfig.leaf.json": `{"compilerOptions": {"composite": true}, "include": ["src"]}`,
				"tsconfig.mid.json":  `{"files": [], "compilerOptions": {"composite": true}, "references": [{"path": "./tsconfig.leaf.json"}]}`,
			},
			owners: map[string]string{
				"src/a.ts": "tsconfig.leaf.json",
			},
		},
		{
			layout: "jsconfig.json where no tsconfig.json exists",
			files: map[string]string{
				"jsconfig.json": `{"compilerOptions": {"baseUrl": "."}}`,
				"src/app.js":    `export const a = 1;`,
				"src/lib.ts":    `export const a = 1;`,
			},
			owners: map[string]string{
				"src/lib.ts": "jsconfig.json",
			},
		},
		{
			layout: "the nearest configuration excludes the file and a parent includes it",
			files: map[string]string{
				"pkg/legacy/old.ts": `export const a = 1;`,
				"pkg/new.ts":        `export const a = 1;`,
				"pkg/tsconfig.json": `{"exclude": ["legacy"]}`,
				"tsconfig.json":     `{"include": ["**/*"]}`,
			},
			owners: map[string]string{
				"pkg/legacy/old.ts": "tsconfig.json",
				"pkg/new.ts":        "pkg/tsconfig.json",
			},
		},
		{
			layout: "the nearest solution misses the file and a parent includes it",
			files: map[string]string{
				"pkg/scripts/x.ts":      `export const a = 1;`,
				"pkg/src/y.ts":          `export const a = 1;`,
				"pkg/tsconfig.app.json": `{"compilerOptions": {"composite": true}, "include": ["src"]}`,
				"pkg/tsconfig.json":     `{"files": [], "references": [{"path": "./tsconfig.app.json"}]}`,
				"tsconfig.json":         `{"include": ["**/*"]}`,
			},
			owners: map[string]string{
				"pkg/scripts/x.ts": "tsconfig.json",
				"pkg/src/y.ts":     "pkg/tsconfig.app.json",
			},
		},
		{
			layout: "tsconfig.json misses the file and jsconfig.json in the same folder includes it",
			files: map[string]string{
				"jsconfig.json": `{"include": ["**/*"]}`,
				"other/b.ts":    `export const a = 1;`,
				"other/c.js":    `export const a = 1;`,
				"src/a.ts":      `export const a = 1;`,
				"tsconfig.json": `{"include": ["src"]}`,
			},
			owners: map[string]string{
				"other/b.ts": "jsconfig.json",
				"src/a.ts":   "tsconfig.json",
			},
		},
		{
			layout: "a file no configuration lists but a listed file imports",
			files: map[string]string{
				"src/lib/x.ts":  `export const x = 1;`,
				"src/lonely.ts": `export const y = 1;`,
				"src/main.ts":   `import { x } from "./lib/x"; export const a = x;`,
				"tsconfig.json": `{"files": ["src/main.ts"]}`,
			},
			owners: map[string]string{
				"src/lib/x.ts":  "tsconfig.json",
				"src/lonely.ts": "",
			},
		},
		{
			layout: "disableSolutionSearching stops the climb",
			files: map[string]string{
				"pkg/legacy/old.ts": `export const a = 1;`,
				"pkg/tsconfig.json": `{"compilerOptions": {"disableSolutionSearching": true}, "exclude": ["legacy"]}`,
				"tsconfig.json":     `{"include": ["**/*"]}`,
			},
			owners: map[string]string{
				"pkg/legacy/old.ts": "",
			},
		},
		{
			layout: "direct references are searched before deeper ones",
			files: map[string]string{
				"src/k.ts":          `export const a = 1;`,
				"tsconfig.a.json":   `{"compilerOptions": {"composite": true}, "include": ["src"]}`,
				"tsconfig.b.json":   `{"compilerOptions": {"composite": true}, "include": ["src"]}`,
				"tsconfig.json":     `{"files": [], "references": [{"path": "./tsconfig.mid.json"}, {"path": "./tsconfig.b.json"}]}`,
				"tsconfig.mid.json": `{"files": [], "compilerOptions": {"composite": true}, "references": [{"path": "./tsconfig.a.json"}]}`,
			},
			owners: map[string]string{
				"src/k.ts": "tsconfig.b.json",
			},
		},
		{
			layout: "tsconfig.json before jsconfig.json",
			files: map[string]string{
				"jsconfig.json": `{"include": ["src"]}`,
				"src/l.ts":      `export const a = 1;`,
				"tsconfig.json": `{"include": ["src"]}`,
			},
			owners: map[string]string{
				"src/l.ts": "tsconfig.json",
			},
		},
		{
			layout: "a parent configuration whose listed file imports the file",
			files: map[string]string{
				"main.ts":           `import { x } from "./pkg/lib/x"; export const a = x;`,
				"pkg/lib/x.ts":      `export const x = 1;`,
				"pkg/src/s.ts":      `export const s = 1;`,
				"pkg/tsconfig.json": `{"include": ["src"]}`,
				"tsconfig.json":     `{"files": ["main.ts"]}`,
			},
			owners: map[string]string{
				"pkg/lib/x.ts": "tsconfig.json",
			},
		},
		{
			layout: "a referenced configuration owns only the files it lists",
			files: map[string]string{
				"src/lib/y.ts":      `export const y = 1;`,
				"src/main.ts":       `import { y } from "./lib/y"; export const a = y;`,
				"tsconfig.app.json": `{"compilerOptions": {"composite": true}, "files": ["src/main.ts"]}`,
				"tsconfig.json":     `{"files": [], "references": [{"path": "./tsconfig.app.json"}]}`,
			},
			owners: map[string]string{
				"src/lib/y.ts": "",
			},
		},
		{
			layout: "a file of a referenced project stays with that project",
			files: map[string]string{
				"configs/shared.json": `{"compilerOptions": {"composite": true}, "include": ["../shared"]}`,
				"shared/z.ts":         `export const z = 1;`,
				"src/main.ts":         `import { z } from "../shared/z"; export const a = z;`,
				"tsconfig.json":       `{"include": ["src"], "references": [{"path": "./configs/shared.json"}]}`,
			},
			owners: map[string]string{
				"shared/z.ts": "configs/shared.json",
			},
		},
		{
			layout: "an import chain through a folder no configuration includes",
			files: map[string]string{
				"lib/a.ts":      `import { b } from "./b"; export const a = b;`,
				"lib/b.ts":      `export const b = 1;`,
				"src/main.ts":   `import { a } from "../lib/a"; export const m = a;`,
				"tsconfig.json": `{"include": ["src"]}`,
			},
			owners: map[string]string{
				"lib/a.ts": "tsconfig.json",
				"lib/b.ts": "tsconfig.json",
			},
		},
		{
			layout: "the import walk follows a paths alias",
			files: map[string]string{
				"src/lib/q.ts":  `export const q = 1;`,
				"src/main.ts":   `import { q } from "@/lib/q"; export const m = q;`,
				"tsconfig.json": `{"compilerOptions": {"paths": {"@/*": ["./src/*"]}}, "files": ["src/main.ts"]}`,
			},
			owners: map[string]string{
				"src/lib/q.ts": "tsconfig.json",
			},
		},
		{
			layout: "include wildcards skip names that start with a dot unless the pattern spells the dot",
			files: map[string]string{
				".hidden/a.ts":  `export const a = 1;`,
				".tools/t.ts":   `export const a = 1;`,
				"lib/.x.ts":     `export const a = 1;`,
				"src/.dot.ts":   `export const a = 1;`,
				"src/b.ts":      `export const a = 1;`,
				"tsconfig.json": `{"include": ["**/*", ".tools/*.ts", "lib/?x.ts"]}`,
			},
			owners: map[string]string{
				".hidden/a.ts": "",
				".tools/t.ts":  "tsconfig.json",
				"lib/.x.ts":    "",
				"src/.dot.ts":  "",
				"src/b.ts":     "tsconfig.json",
			},
		},
		{
			layout: "brackets in include patterns are plain characters",
			files: map[string]string{
				"app/[id]/page.ts": `export const a = 1;`,
				"lib/[ab].ts":      `export const a = 1;`,
				"lib/a.ts":         `export const a = 1;`,
				"tsconfig.json":    `{"include": ["app/[id]/*.ts", "lib/[ab].ts"]}`,
			},
			owners: map[string]string{
				"app/[id]/page.ts": "tsconfig.json",
				"lib/[ab].ts":      "tsconfig.json",
				"lib/a.ts":         "",
			},
		},
	} {
		got := ownersIn(t, tc.files)
		for file, want := range tc.owners {
			if got[file] != want {
				t.Errorf("%s: %s is owned by %q, want %q", tc.layout, file, got[file], want)
			}
		}
	}
}

// TestExcludedTestFileResolvesThroughItsOwnConfig covers the layout of
// the TypeScript 5.7 release notes with aliases: the nearest
// tsconfig.json excludes the test, so its alias resolves through the
// test configuration the root solution references.
func TestExcludedTestFileResolvesThroughItsOwnConfig(t *testing.T) {
	root, files := writeFiles(t, map[string]string{
		"tsconfig.json":          `{"files": [], "references": [{"path": "./src/tsconfig.json"}, {"path": "./src/tsconfig.test.json"}]}`,
		"src/tsconfig.json":      `{"compilerOptions": {"paths": {"@/*": ["./*"]}}, "exclude": ["**/*.test.ts"]}`,
		"src/tsconfig.test.json": `{"compilerOptions": {"paths": {"@test/*": ["./*"]}}, "include": ["**/*.test.ts"]}`,
		"src/order.ts":           `import { a } from "@/money";` + "\n",
		"src/money.ts":           "export const a = 1;\n",
		"src/order.test.ts":      `import { b } from "@test/order";` + "\n" + `import { c } from "@/money";` + "\n",
	})
	facts := produce(t, root, files)
	expectTarget(t, importOf(t, facts, "src/order.ts", "@/money"), conformance.ImportInternal, "src/money.ts", "src")
	expectTarget(t, importOf(t, facts, "src/order.test.ts", "@test/order"), conformance.ImportInternal, "src/order.ts", "src")
	expectTarget(t, importOf(t, facts, "src/order.test.ts", "@/money"), conformance.ImportUnknown, "", "")
}
