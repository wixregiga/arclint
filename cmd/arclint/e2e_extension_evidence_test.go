package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

const reverseDependencyExtension = `
import { defineRule } from "arclint";
export default defineRule({
  type: "imported-only-by",
  check(ctx) {
    const subjects = ctx.scope().files;
    for (const importer of ctx.files("**/*.go")) {
      if (ctx.zoneOf(importer.path).includes("cli_factory")) continue;
      for (const imp of ctx.imports(importer.path)) {
        const subjectPath = subjects.find(path => imp.targetFile
          ? path === imp.targetFile
          : path.slice(0, path.lastIndexOf("/")) === imp.targetDir);
        if (subjectPath) ctx.report({
          subjectPath, path: importer.path, line: imp.line,
          message: importer.path + " may not import cobra_adapter",
        });
      }
    }
  }
});
`

func TestExtensionReverseDependencyEvidence(t *testing.T) {
	for _, tc := range []struct {
		name, selection string
		unauthorized    bool
		wantViolations  int
	}{
		{"unauthorized and authorized", "    on: cobra_adapter\n", true, 1},
		{"authorized only", "    on: cobra_adapter\n", false, 0},
		{"file narrowed", "    on: cobra_adapter\n    files: '**/cobra.go'\n", true, 1},
		{"path excluded", "    on: cobra_adapter\n    exclude:\n      paths: [internal/adapter/legacy.go]\n      reason: legacy file\n", true, 1},
		{"importer exclusion keeps evidence", "    on: cobra_adapter\n    exclude:\n      paths: [internal/application/**]\n      reason: governs adapter only\n", true, 1},
		{"zone excluded", "    on: cobra_adapter\n    exclude:\n      zones: [cobra_adapter]\n      reason: deferred adapter\n", true, 0},
		{"empty scope", "    on: cobra_adapter\n    files: '**/absent.go'\n", true, 0},
		{"repository file scope", "    files: internal/adapter/**\n", true, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, "go.mod", "module example.com/project\n\ngo 1.26\n")
			write(t, root, "rules.arclint.yaml", `runtime: [go]
zones:
  cobra_adapter: internal/adapter/**
  cli_factory: internal/factory/**
  application: internal/application/**
rules:
  delivery/cobra-factory-only:
    uses: imported-only-by
`+tc.selection)
			write(t, root, ".arclint/extensions/imported_only_by.ts", reverseDependencyExtension)
			write(t, root, "internal/adapter/cobra.go", "package adapter\n")
			write(t, root, "internal/adapter/legacy.go", "package adapter\n")
			write(t, root, "internal/factory/factory.go", "package factory\nimport _ \"example.com/project/internal/adapter\"\n")
			if tc.unauthorized {
				write(t, root, "internal/application/run.go", "package application\nimport _ \"example.com/project/internal/adapter\"\n")
			}
			stdout, stderr, code := runBin(t, root, os.Environ(), "check", "--format", "json")
			wantExit := 0
			if tc.wantViolations > 0 {
				wantExit = 1
			}
			if code != wantExit {
				t.Fatalf("exit=%d want=%d\n%s\n%s", code, wantExit, stdout, stderr)
			}
			var diagnostics []diagnosticDoc
			if err := json.Unmarshal([]byte(stdout), &diagnostics); err != nil {
				t.Fatal(err)
			}
			count := 0
			for _, d := range diagnostics {
				if d.Kind == "operational" {
					t.Fatalf("unexpected operational diagnostic: %+v", d)
				}
				if d.Kind != "violation" {
					continue
				}
				count++
				if d.RuleID != "delivery/cobra-factory-only" || d.Path != "internal/application/run.go" || d.Line != 2 || !strings.Contains(d.Message, "may not import cobra_adapter") {
					t.Fatalf("unexpected attribution: %+v", d)
				}
			}
			if count != tc.wantViolations {
				t.Fatalf("violations=%d want=%d\n%s", count, tc.wantViolations, stdout)
			}
		})
	}
}

func TestExtensionForwardCheckUsesResolvedScope(t *testing.T) {
	root := t.TempDir()
	write(t, root, "rules.arclint.yaml", `runtime: [go]
zones:
  src: src/**
rules:
  forward:
    on: src
    files: '**/*.go'
    uses: forward
    exclude:
      paths: [src/excluded.go]
      reason: legacy file
`)
	write(t, root, ".arclint/extensions/forward.ts", `
import { defineRule } from "arclint";
export default defineRule({type: "forward", check(ctx) {
  for (const path of ctx.scope().files) {
    if (ctx.read(path).includes("forbidden")) ctx.report({path, message: "forbidden content"});
  }
}});
`)
	for _, path := range []string{"src/dirty.go", "src/excluded.go", "outside/dirty.go", "src/note.txt"} {
		write(t, root, path, "package src\n// forbidden\n")
	}
	write(t, root, "src/clean.go", "package src\n")
	stdout, stderr, code := runBin(t, root, os.Environ(), "check", "--format", "json")
	if code != 1 {
		t.Fatalf("exit=%d\n%s\n%s", code, stdout, stderr)
	}
	var diagnostics []diagnosticDoc
	if err := json.Unmarshal([]byte(stdout), &diagnostics); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, d := range diagnostics {
		if d.Kind == "operational" {
			t.Fatalf("unexpected operational diagnostic: %+v", d)
		}
		if d.Kind == "violation" {
			count++
			if d.Path != "src/dirty.go" || d.RuleID != "forward" {
				t.Fatalf("forward scope changed: %+v", d)
			}
		}
	}
	if count != 1 {
		t.Fatalf("violations=%d\n%s", count, stdout)
	}
}
