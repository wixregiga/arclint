# arclint

A repository linter and architectural-conformance system. One static
Go binary evaluates **Rules** against Files, Folders, and Modules, then
reports what it could prove, what it suspects, and what it could not
determine — silence never reads as conformance.

Rules are plain YAML. Every Rule has an explicit stable id, states one
claim, and declares how it is enforced: evidence, assurance, and
limitations are reported alongside every finding.

## Quickstart

```bash
make build          # CGO_ENABLED=0, single static binary
./arclint init      # draft a commented starter rules.yaml
./arclint check .   # evaluate it
```

Grow the starter module by module. A real contract set looks like:

```yaml
runtime: [go]

modules:
  domain:
    paths: ["internal/domain/**"]
    description: "Aggregates and domain values; stdlib-only."
  application:
    paths: ["internal/application/**"]
    description: "Use cases coordinating domain objects through ports."

contracts:
  domain:
    consumes:
      id: "repo:domain/stdlib-only"
      internal: []        # may import no other declared module
      external: forbid
  application:
    consumes:
      id: "repo:application/inward"
      internal: [domain]
      external: forbid

dependencies:
  - id: "repo:dependencies/acyclic"
    kind: acyclic
```

## Rule types

The finite, arclint-owned set — extensions plug into it, they never
extend it:

| kind | claim shape |
|---|---|
| `consumes` | what a module may import: internal allow-list, external and stdlib policy |
| `structure` | files a module must contain or must not contain (globs) |
| `naming` | file-name case vocabulary (`snake_case`, `kebab-case`, `camelCase`, `PascalCase`, `regex:`) |
| `layers` | modules ordered highest first; imports go same-or-lower only |
| `protected` | who may import one module |
| `acyclic` | no dependency cycles among declared modules |
| `extension` | enforcement supplied by a TypeScript extension via `uses` + `with` |

Import analysis is exact, never heuristic: Go classification follows
the toolchain (embedded `go list std` table, module-path ownership,
replace directives, go.work), TypeScript and Python follow their
manifests, and each shipped language adapter embeds a generated stdlib
table — an obligation the ruleset itself enforces.

## Honest reporting

Every evaluation ends in exactly one outcome: `conforms`, `violates`,
`suspected_violation`, `undetermined`, `unsupported`, `not_applicable`,
or `failed`. Severity (does it gate?) is independent from assurance
(how sure is the evidence?). Unsupported and failed evaluations surface
as coverage and operational diagnostics instead of disappearing.

Exit codes: `0` clean, `1` error-severity findings, `2` configuration
or usage error.

Adopted debt lives in a committed, reviewable baseline
(`.arclint/baseline.v2.json`): `baseline capture` adopts current
findings, `check` reports only new ones and counts the covered rest,
`baseline refresh` drops entries that no longer occur.

## Commands

```
check [path]        evaluate the repository (--format human|json, --no-baseline)
rules [selector]    list the configured rules; one match shows the complete rule
context <path>      modules, rules, and reasons binding a location (--format json)
agents [--write]    compile the ruleset into a generated AGENTS.md block
baseline capture    adopt current findings   ·  baseline refresh: drop stale entries
patterns            list local pattern packages (.arclint/patterns/<name>/pattern.yaml)
sdk init            write arclint.d.ts + tsconfig.json for extension authors
init                draft a starter rules.yaml (--languages go,ts,py --force)
```

## Extensions

Custom enforcement is TypeScript under `.arclint/extensions/`,
transpiled in-process by esbuild and executed on a deterministic
sandbox (sobek): no npm, no Node, no filesystem or network access, and
host-controlled clock and randomness. Parameters are validated
host-side against the schema the extension publishes, before any
extension code runs. The complete showcase lives in this repository —
[.arclint/extensions/forbid_content.ts](.arclint/extensions/forbid_content.ts)
enforces `arclint:domain/no-panic` from rules.yaml:

```yaml
- id: "arclint:domain/no-panic"
  kind: extension
  files: "internal/domain/**/*.go"
  uses: forbid-content
  with:
    pattern: '\bpanic\('
```

Extensions see exactly the subjects the rule's applicability selects;
findings outside that scope are rejected. The capability surface is
one interface for every language: `ctx.files`, `ctx.read`,
`ctx.imports`, and `ctx.facts` — the same normalized shapes for Go,
TypeScript, and Python. Declarations use a closed cross-language
vocabulary: go/parser exact for Go, pinned tree-sitter grammars for
TypeScript and Python (embedded per the Makefile's grammar-subset
tags), deterministic everywhere. Extension evidence is treated as heuristic:
findings gate as suspected violations, and absence of findings is
undetermined, never proof of conformance.

## Self-hosting

This repository is checked by its own ruleset on every CI run
(`make selfcheck`). The codebase follows the architecture it enforces:

- `internal/domain` — the Rule aggregate, conformance check, and
  baseline model; stdlib-only, invalid values unconstructible.
- `internal/application` — action-named use cases behind ports.
- `internal/infrastructure` — outbound adapters: ruleset loading,
  observation, language facts, baselines, patterns, the extension
  runtime.
- `internal/delivery` — the framework-neutral CLI; Cobra is sealed
  inside one adapter behind a factory.
- `cmd/arclint` — the composition root.
- Verification lives beside what it verifies: the Go adapter carries
  a toolchain suite proving classification against `go list` over
  pinned real repositories (part of the normal test run, skipped under
  `-short`), and the domain glob matcher carries a committed reference
  truth table.

## Developing

```bash
make ci                  # vet + tests (ground truth included) + selfcheck
go test -short ./...     # the quick loop, network suite skipped
make bench               # cold start and large-repo timings
```

The docs site under `docs/site` predates the current engine and is
awaiting a rewrite; this README and the generated
[AGENTS.md](AGENTS.md) are the current references.
