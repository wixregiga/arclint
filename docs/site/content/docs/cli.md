+++
title = "CLI"
description = "Every command, with what it reads and writes."
weight = 6
+++

| command | does |
|---|---|
| `arclint init` | interactive setup: detect languages, pick a pattern, write rules.yaml + extensions + typings, validate. Flags: `--runtimes go,ts`, `--pattern <name>`, `--force` |
| `arclint patterns` | list architectural patterns, builtin and `.arclint/patterns/`. `--extensions` also lists each pattern's extension files |
| `arclint check [path]` | evaluate the contracts; `--format json` for the stable violation shape; `--show-suppressed` also lists findings dropped by except clauses, with reasons |
| `arclint load [rules.yaml]` | parse, validate (schema + semantics + extension params), cache; prints what loaded |
| `arclint list` | one line per loaded rule |
| `arclint rules ls` | rule table: id, contract, kind, module, provider, severity, description |
| `arclint module ls` | declared modules with file counts, languages, descriptions |
| `arclint module info <name>` | one module: description, paths, members, every rule that binds it |
| `arclint explain [kind]` | terminal docs for any rule kind or extension type |
| `arclint rules show <id\|ns>` | every clause grouped under one rule id or namespace prefix, with its exceptions; `--format json` |
| `arclint rules test [paths]` | run rule test cases (default `.arclint/tests`); `--pattern <name>` runs a pattern's bundled suite |
| `arclint sdk init` | write `arclint.d.ts` + `tsconfig.json` for extension authoring |

Exit codes everywhere: `0` clean, `1` error-severity violations, `2`
configuration or usage error.

## The violation shape

`--format json` emits an array with a stable contract:

```json
{
  "ruleId": "deps.shared-only-via-app",
  "contract": "consumes",
  "blame": "consumer",
  "severity": "error",
  "capability": "exact",
  "path": "internal/borrowbook/sneaky.go",
  "line": 3,
  "message": "import resolves to protected module \"shared\"",
  "fixHint": "route the dependency through app"
}
```

`ruleId` is stable across runs (explicit `id:` wins; defaults derive
from the module and kind). `line` is present when the violation anchors
to a line.
