+++
title = "CLI"
description = "Every command, with what it reads and writes."
weight = 6
+++

| command | does |
|---|---|
| `arclint init` | interactive setup: detect languages, pick a pattern, write rules.yaml + extensions + typings, validate. Flags: `--runtimes go,ts`, `--pattern <name>`, `--force` |
| `arclint patterns` | list architectural patterns, builtin and `.arclint/patterns/`. `--extensions` also lists each pattern's extension files |
| `arclint check [path]` | evaluate the contracts; `--format json\|line\|sarif` for machine shapes; `--show-suppressed` also lists findings dropped by except clauses, with reasons; `--only <id\|ns>` restricts findings (and the exit code) to one rule id or namespace |
| `arclint load [rules.yaml]` | parse, validate (schema + semantics + extension params), cache; prints what loaded, lists extension types registered but not instantiated, and warns on instances whose params are empty lists (armed but inert) |
| `arclint list` | one line per loaded rule |
| `arclint facts <file>` | the declaration facts a rule sees from `ctx.facts(path)`: kinds, names, owners, signatures; `--format json` for the exact wire shape |
| `arclint rules ls` | rule table: id, contract, kind, module, provider, severity, description |
| `arclint module ls` | declared modules with file counts, languages, descriptions |
| `arclint module info <name>` | one module: description, paths, members, every rule that binds it |
| `arclint explain [kind]` | terminal docs for any rule kind or extension type |
| `arclint rules show <id\|ns>` | every clause grouped under one rule id or namespace prefix, with its exceptions; `--format json` |
| `arclint rules test [paths]` | run rule test cases (default `.arclint/tests`); `--pattern <name>` runs a pattern's bundled suite |
| `arclint rules scaffold <type>` | stub extension + failing test case + rules.yaml snippet: the red-first start of a new rule |
| `arclint sdk init` | write `arclint.d.ts` + `tsconfig.json` for extension authoring |

Exit codes everywhere: `0` clean, `1` error-severity violations, `2`
configuration or usage error.

## Shell completion

`arclint completion bash|zsh|fish|powershell` emits the script.
Completion is dynamic: TAB completes module names for `module info`,
rule ids and namespaces for `rules show` and `check --only`, pattern
names for `init --pattern` and `rules test --pattern`, rule kinds for
`explain`, and closed value sets for `--format`. Values come from the
rules.yaml the current directory resolves to; without one, value
completion stays silent.

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

## Editor and CI formats

`--format line` prints one finding per line for regex-based toolchains
(VS Code problemMatcher, vim errorformat):

```text
internal/borrowbook/sneaky.go:3: error: import resolves to protected module "shared" [deps.shared-only-via-app]
```

An unanchored finding prints line 0; editors clamp it to the top of the
file. Suppressed findings never appear in the line format: editors show
problems, not policy.

`--format sarif` emits SARIF 2.1.0 for GitHub code scanning and the VS
Code SARIF Viewer. Findings carry a stable `partialFingerprints` entry
(rule, path, message — line moves do not reopen findings), and under
`--show-suppressed` the excepted findings appear with a SARIF
suppressions block carrying the except reason. Contract, blame,
capability, and fixHint ride in each result's property bag.
