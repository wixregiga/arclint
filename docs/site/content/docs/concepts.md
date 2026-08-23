+++
title = "Concepts"
description = "Modules, contracts, Assurance, and how ArcLint reports findings."
weight = 2
+++

## Modules

A Module is a named set of files, defined by path globs in `rules.yaml`.
Modules are the vocabulary other Rules use: consumes allow-lists,
layers, and protections refer to Module names, never raw paths.

```yaml
modules:
  entities:
    paths: ["internal/entities/**"]
    description: "Domain types and invariants; depends on nothing."
  features:
    paths: ["internal/features/*"]
```

A glob matches files directly, and a glob naming a directory owns the
whole subtree. Overlapping Modules are legal: a file can belong to
several Modules, which makes umbrella Modules
(`paths: ["internal/**"]`) cheap for repo-wide invariants.

Inspect the loaded map with `arclint context` or
`arclint context --module <name>`.

## Import classes

Every import in every scanned file is classified before dependency
Rules run. The class names appear throughout the Rule surface:

| class | meaning |
|---|---|
| `internal` | resolves to a file inside this repository: another Module, or undeclared internal code |
| `external` | a third-party dependency declared in your manifest: `go.mod` require, `package.json` dependencies, `pyproject.toml` |
| `stdlib` | the language's standard library (embedded tables generated from each toolchain) |
| `unknown` | none of the above; governed by `scan.unknown_imports: warn/error/ignore` |

So in a consumes Rule, `internal: [app]` means "may import the app
Module and nothing else internal", and `external: forbid` means "no
third-party libraries here at all". Go classification follows toolchain
semantics. TypeScript and Python are lexer-grade with documented
limits: computed specifiers like `import(x)` or
`importlib.import_module(name)` are invisible by design.

## Contracts and dependencies

`rules.yaml` binds Rules in two places:

- **`contracts.<module>`** — Rules scoped to one declared Module:
  - `consumes`: what the Module may depend on (internal allow-list,
    external and stdlib policy).
  - `invariants`: properties that always hold over member files —
    `structure`, `naming`, and `extension` Rule Types.
- **`dependencies:`** — repository-wide graph Rules that span Modules:
  `layers`, `protected`, and `acyclic`.

The [rule reference](/docs/rules/) lists every published Rule Type and
paste-ready YAML. `arclint rules` lists configured Rules;
`arclint rules <id>` shows one complete Rule when the selector matches
exactly.

## Assurance

Every Rule Type states how strongly Enforcement can decide its Claim.
Findings and Rule detail carry the label:

| Assurance | basis |
|---|---|
| `exact` | fully decides the Claim within a documented analysis limit |
| `partial` | reported Violations are trustworthy, but some cases may be unobservable |
| `heuristic` | may produce false positives or false negatives |
| `advisory` | guidance without automated truth judgment |

Builtin import and tree Rules use `exact`. Extension Rules are
`heuristic`: the engine treats extension evidence as heuristic
regardless of what an Extension declares. Severity (`error`,
`warning`, `info`) is configured on the Rule and is independent from
Assurance.

## Rule identity

Rule IDs are stable strings. A material Claim change needs a new ID.
Patterns may namespace IDs (`slice:…`, `layers:…`) so a selector can
narrow a distributed set. `arclint rules <selector>` lists matches or
shows one Rule; `arclint rules test` runs fixture-backed Rule Tests
under `.arclint/tests`.

## Baseline

The Baseline is the adoption tool for debt that is acknowledged but
must not grow. `arclint baseline capture` records current active
findings in `.arclint/baseline.v2.json` (commit it). `check` then reports
only new findings, always prints the baselined count, and
`arclint baseline refresh` replaces the snapshot after comparison so
stale entries drop as debt is paid. Entries key on a fingerprint of
Rule, subject, and message, so line moves do not reopen findings, and
identical findings carry a count.

`check --no-baseline` evaluates without subtracting the file. The file
itself is reviewable: every entry carries the finding it covers, not
just a hash, and it contains no timestamps, so regenerating it diffs
only when findings change.

## Project domain model

The project's Ubiquitous Language lives in a committed
`ubiquitous-language.yaml` beside `rules.yaml`. It is first-class
project knowledge—not hidden ArcLint machinery—organized as a structured
Project Domain Model of Entities, Aggregates, Value Objects, Business
Rules, and Domain Events. ArcLint owns the meanings of those concepts;
the project supplies names, definitions, and aliases.

| concept | meaning |
|---|---|
| Entity | A domain concept whose identity matters as it changes over time. |
| Aggregate | An Entity the project treats as a consistency boundary: it is changed as one unit and other objects reach it through its identity. |
| Value Object | A domain value defined entirely by its attributes, with no identity of its own. |
| Business Rule | A statement the project requires to always or never be true about its domain. |
| Domain Event | Something that has completed in the domain and that the project cares to record. |

An Aggregate is an Entity designation (`aggregate: true` on an entity
entry), not a second unrelated object. Inspect and maintain the model
with `arclint domain`; start an empty model with `arclint domain init`.
Initialization leaves an existing file untouched. `arclint domain explain`
prints the same ArcLint meanings used by help, guided authoring, JSON output,
and the extension SDK. Declaring knowledge never creates a Diagnostic by
itself—enabled Rules under `arclint check` decide whether the model is enforced.

## Validation layers

`rules.yaml` passes three gates before anything runs: YAML syntax, the
published JSON Schema (the same file that powers editor completion and
`arclint rules schema`), and semantic validation (Module references,
regex compilation, Extension parameter schemas). Extension Rule params
are validated against each Extension's declared schema before a line of
extension code executes.

## Exit codes

`0` is a clean command. `1` means the gate failed: an active
error-severity Violation or an error-severity operational Diagnostic
(or a Rule Test expectation mismatch). `2` is configuration or usage
error.
