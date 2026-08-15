+++
title = "CLI"
description = "Every command, with what it reads and writes."
weight = 6
+++

| command | does |
|---|---|
| `arclint init` | interactive setup: detect languages, pick a pattern, write rules.yaml + extensions + typings, validate. Flags: `--runtimes go,ts`, `--pattern <name>`, `--force` |
| `arclint patterns` | list architectural patterns, builtin and `.arclint/patterns/`. `--extensions` also lists each pattern's extension files |
| `arclint check [path]` | evaluate the contracts; `--format json\|line\|sarif` for machine shapes; `--show-suppressed` and `--show-baselined` also list omitted findings; `--no-baseline` ignores the committed baseline; `--only <id\|ns>` restricts findings (and the exit code) to one rule id or namespace |
| `arclint baseline [path]` | adopt current findings into `.arclint/baseline.json` (commit it); check then reports only new findings, counts the debt visibly, and warns when entries go stale |
| `arclint load [rules.yaml]` | parse, validate (schema + semantics + extension params), cache; prints what loaded, lists extension types registered but not instantiated, and warns on instances whose params are empty lists (armed but inert) |
| `arclint list` | one line per loaded rule |
| `arclint facts <file>` | the declaration facts a rule sees from `ctx.facts(path)`: kinds, names, owners, signatures; `--format json` for the exact wire shape |
| `arclint context <path\|module>` | the architectural context of one location: owning modules, allowed internal imports, external/stdlib policy, every binding rule, and the verify command; `--format json` for agents |
| `arclint agents` | compile the ruleset into a compact AGENTS.md architecture block; prints by default, `--write` installs or refreshes it between markers in `<repo-root>/AGENTS.md`, preserving everything outside them |
| `arclint rules ls` | rule table: id, contract, kind, module, provider, severity, description |
| `arclint module ls` | declared modules with file counts, languages, descriptions |
| `arclint module info <name>` | one module: description, paths, members, every rule that binds it |
| `arclint explain [kind]` | terminal docs for any rule kind or extension type |
| `arclint rules show <id\|ns>` | every clause grouped under one rule id or namespace prefix, with its exceptions; `--format json` |
| `arclint rules test [paths]` | run rule test cases (default `.arclint/tests`); `--pattern <name>` runs a pattern's bundled suite |
| `arclint rules scaffold <type>` | stub extension + failing test case + rules.yaml snippet: the red-first start of a new rule |
| `arclint sdk init` | write `arclint.d.ts` + `tsconfig.json` for extension authoring |

Exit codes everywhere: `0` clean, `1` error-severity violations, `2`
configuration or usage error. Every command that reads a ruleset
accepts `--rules <path>` to name rules.yaml explicitly; the default is
discovery upward from the working directory, and the rules.yaml
directory is the repo root and the extension root.

## Context for agents

`arclint context <path|module>` answers "what is architecturally true
where I am about to edit?" without loading the whole ruleset into a
prompt: the modules owning the path, their descriptions, what they may
import, every rule binding them, and the command that verifies the
result. A file path, a directory, or a declared module name all
resolve; an exact module name wins when both match. `--format json`
emits the machine shape for coding agents.

`arclint agents --write` covers the prompt-time half: it compiles the
ruleset into a generated block inside `AGENTS.md` (modules, dependency
policy, repo-wide rules, query commands) so agents see the architecture
before writing code. The block sits between markers; hand-written
content around it survives regeneration, and the block never carries
timestamps, so regeneration is idempotent for an unchanged ruleset.

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
