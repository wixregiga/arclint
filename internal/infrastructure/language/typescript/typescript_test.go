package typescript

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
)

// writeFiles materializes a fixture tree and returns the root plus the
// sorted observed-file list the walk would produce.
func writeFiles(t *testing.T, files map[string]string) (string, []conformance.ObservedFile) {
	t.Helper()
	root := t.TempDir()
	var observed []conformance.ObservedFile
	for p, content := range files {
		abs := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		observed = append(observed, conformance.ObservedFile{Path: p, Size: int64(len(content))})
	}
	sort.Slice(observed, func(i, j int) bool { return observed[i].Path < observed[j].Path })
	return root, observed
}

func produce(t *testing.T, root string, files []conformance.ObservedFile) map[string]conformance.LanguageFacts {
	t.Helper()
	facts, err := NewProducer().Facts(root, files, nil)
	if err != nil {
		t.Fatalf("Facts: %v", err)
	}
	return facts
}

func TestFactsClassification(t *testing.T) {
	root, files := writeFiles(t, map[string]string{
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
	facts := produce(t, root, files)
	fa, ok := facts["src/main.ts"]
	if !ok {
		t.Fatal("main.ts not analyzed")
	}
	if fa.Language != rule.LanguageTypeScript || !fa.ImportsAvailable || fa.ParseFailure != "" {
		t.Fatalf("facts header: %+v", fa)
	}
	type want struct {
		spec  string
		class conformance.ImportClass
		tf    string
		td    string
		line  int
	}
	wants := []want{
		{"node:fs", conformance.ImportStdlib, "", "", 1},
		{"path", conformance.ImportStdlib, "", "", 2},
		{"lodash", conformance.ImportExternal, "", "", 3},
		{"@types/node/fs", conformance.ImportExternal, "", "", 4},
		{"./util", conformance.ImportInternal, "src/util.ts", "src", 5},
		{"../src/lib/index", conformance.ImportInternal, "src/lib/index.ts", "src/lib", 6},
		{"./nowhere", conformance.ImportInternal, "", "", 7},
		{"unknown-pkg", conformance.ImportUnknown, "", "", 8},
	}
	if len(fa.Imports) != len(wants) {
		t.Fatalf("imports: %+v", fa.Imports)
	}
	for i, w := range wants {
		got := fa.Imports[i]
		if got.Path != w.spec || got.Class != w.class || got.TargetFile != w.tf || got.TargetDir != w.td || got.Line != w.line {
			t.Errorf("[%d] = {%s %s tf=%q td=%q line=%d}, want {%s %s tf=%q td=%q line=%d}",
				i, got.Path, got.Class, got.TargetFile, got.TargetDir, got.Line,
				w.spec, w.class, w.tf, w.td, w.line)
		}
	}
}

func TestMonorepoWorkspaceNameResolvesInternal(t *testing.T) {
	root, files := writeFiles(t, map[string]string{
		"package.json":               `{"name": "root", "workspaces": ["packages/*"]}`,
		"packages/core/package.json": `{"name": "@app/core"}`,
		"packages/web/package.json":  `{"name": "@app/web", "dependencies": {"@app/core": "workspace:*", "react": "^18"}}`,
		"packages/web/index.ts":      `import { boot } from "@app/core";` + "\n" + `import React from "react";` + "\n",
		"packages/core/index.ts":     "export const boot = 1;\n",
	})
	facts := produce(t, root, files)
	fa, ok := facts["packages/web/index.ts"]
	if !ok || len(fa.Imports) != 2 {
		t.Fatalf("imports: %+v", fa)
	}
	if fa.Imports[0].Class != conformance.ImportInternal || fa.Imports[0].TargetDir != "packages/core" {
		t.Errorf("workspace name: %+v", fa.Imports[0])
	}
	if fa.Imports[1].Class != conformance.ImportExternal {
		t.Errorf("react: %+v", fa.Imports[1])
	}
}

func TestNestedManifestNearestWins(t *testing.T) {
	root, files := writeFiles(t, map[string]string{
		"package.json":          `{"dependencies": {"root-only": "1"}}`,
		"apps/api/package.json": `{"dependencies": {"api-only": "1"}}`,
		"apps/api/server.ts":    `const a = require("api-only");` + "\n" + `const r = require("root-only");` + "\n",
	})
	facts := produce(t, root, files)
	fa := facts["apps/api/server.ts"]
	if fa.Imports[0].Class != conformance.ImportExternal {
		t.Errorf("api-only: %+v", fa.Imports[0])
	}
	// root-only is not declared by the nearest manifest: unknown, matching
	// the nested go.mod ownership rule.
	if fa.Imports[1].Class != conformance.ImportUnknown {
		t.Errorf("root-only: %+v", fa.Imports[1])
	}
}

// TestOnlyOwnedFilesClaimed pins the target vocabulary: the producer
// claims .ts/.tsx (minus .d.ts) and never the .js family the legacy
// analyzer also covered — while relative resolution still probes .js
// targets.
func TestOnlyOwnedFilesClaimed(t *testing.T) {
	root, files := writeFiles(t, map[string]string{
		"types.d.ts":   `import "phantom";`,
		"real.ts":      `import "node:fs";` + "\n" + `import legacy from "./old";` + "\n",
		"widget.tsx":   `import "node:path";`,
		"old.js":       "module.exports = 1;\n",
		"script.mjs":   `import "node:os";`,
		"commonjs.cjs": `require("node:os");`,
		"legacy.jsx":   `import "node:os";`,
	})
	facts := produce(t, root, files)
	for _, unclaimed := range []string{"types.d.ts", "old.js", "script.mjs", "commonjs.cjs", "legacy.jsx"} {
		if _, ok := facts[unclaimed]; ok {
			t.Errorf("%s claimed; TypeScript owns only .ts/.tsx", unclaimed)
		}
	}
	for _, claimed := range []string{"real.ts", "widget.tsx"} {
		if _, ok := facts[claimed]; !ok {
			t.Errorf("%s not claimed", claimed)
		}
	}
	real := facts["real.ts"]
	if len(real.Imports) != 2 {
		t.Fatalf("real.ts imports: %+v", real.Imports)
	}
	if real.Imports[1].Class != conformance.ImportInternal || real.Imports[1].TargetFile != "old.js" {
		t.Errorf("relative import into .js target: %+v", real.Imports[1])
	}
}

func TestStdlibTableSanity(t *testing.T) {
	for _, m := range []string{"fs", "node:fs", "path", "fs/promises", "child_process"} {
		if !isStdlib(m) {
			t.Errorf("%s missing from Node builtin table", m)
		}
	}
	for _, m := range []string{"lodash", "react", "@scope/x"} {
		if isStdlib(m) {
			t.Errorf("%s wrongly stdlib", m)
		}
	}
}
