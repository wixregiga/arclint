+++
title = "Concepts"
description = "Modules, Rules, Patterns, Assurance, and how ArcLint reports findings."
weight = 2
+++

## Modules

A Module is a named set of files, defined by path globs in `rules.yaml`.
Modules are the vocabulary other Rules use: consumes allow-lists,
layers, and protections refer to Module names, never raw paths.

A Module is logical, not a folder. One Module may gather files from
many roots, and one glob may reach into every slice of a vertically
sliced tree: `internal/*/domain/**` is the domain layer of every
feature, wherever the feature lives.

```yaml
modules:
  entities:
    paths: "internal/*/domain/**"
    description: "Every slice's domain layer; depends on nothing."
  use_cases: "internal/*/app/**"
  transport: ["internal/*/http/**", "internal/*/grpc/**"]
  toolchain: ["Makefile", "go.mod"]
```

A glob matches files directly, and a glob naming a directory owns the
whole subtree. Overlapping Modules are legal: a file can belong to
several Modules, which makes umbrella Modules (`source: "internal/**"`)
cheap for repo-wide invariants.

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

So in an `imports` Rule, `internal: [app]` means "may import the app
Module and nothing else internal", and `external: forbid` means "no
third-party libraries here at all". Go classification follows toolchain
semantics. TypeScript and Python are lexer-grade with documented
limits: computed specifiers like `import(x)` or
`importlib.import_module(name)` are invisible by design.

## Rules and assertions

`rules:` is one map keyed by Rule ID. Every Rule states one Claim
(`description`), judges the Modules named under `on`, and carries
exactly one assertion key; that key is the Rule Type:

- Module-scoped: `imports` (what the Module may depend on),
  `structure` (files it must or must not contain), `naming`,
  `content` (lines it must not contain), `invariants` (recorded domain
  contracts visible in source), and `uses` (an Extension).
- Graph-scoped, with the Modules in the assertion itself: `layers`,
  `imported_by` (who may import the one Module under `on`),
  `independent`, and `acyclic`.

A Rule with two assertion keys is rejected: give each claim its own ID.
The [rule reference](/docs/rules/) lists every published Rule Type and
paste-ready YAML. `arclint rules` lists configured Rules;
`arclint rules <id>` shows one complete Rule when the selector matches
exactly.

## Patterns and adoption

A Pattern distributes Rules by reference. `extends` names it by exact
version and binds every Pattern Module to local paths; the Pattern's
Rules load under the Pattern's namespace, and an entry under `rules:`
with no assertion is an Override of one of them: `severity`, `disable`
with a reason, `exclude`, or `suppress`. Nothing is copied.

A Pattern resolves offline first: from the binary that embeds it, then
from a vendored or authored copy under `.arclint/patterns`. A Registry
is read only when `install` or `vendor` asks for a Pattern that
resolves nowhere offline, and the copy it writes is verified against
its manifest on every load; a check never reaches the network. See
[Patterns](/docs/patterns/).

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

Rule IDs are stable strings of the form `segment/segment`
(`domain/stdlib-only`). A material Claim change needs a new ID. Rules an
extended Pattern distributes carry the Pattern's namespace
(`arclint:domain/stdlib-only`), so the prefix selector `arclint:` narrows
to the distributed set. `arclint rules <selector>` lists matches or
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
project knowledge, not hidden ArcLint machinery. The file is organized
by bounded context: each context holds entities, value objects,
invariants, assertions, specifications, and events; top-level relations name how contexts connect.
ArcLint owns the meanings of the supported concepts; the project
supplies names, definitions, aliases, invariant statements, and owners.

| concept | spelling | meaning |
|---|---|---|
| Entity | `entity` | A domain concept whose identity matters as it changes over time. |
| Aggregate | `aggregate` / `aggregate_root` | An Entity designation (`aggregate: true`): a consistency boundary reached through its identity. |
| Value Object | `value_object` | A domain value defined entirely by its attributes, with no identity of its own. |
| Domain Event | `domain_event` | Something that has completed in the domain and that the project cares to record (file section: `events`). |
| Bounded Context | `bounded_context` | A linguistic boundary; terms are defined inside one context. |

The published JSON Schema for the file is generated by the binary and
committed at `.agents/skills/domain-librarian/library.schema.json` (also
printed by `arclint domain schema`). Inspect and maintain the model with
`arclint domain`; start an empty model with `arclint domain init`.
Initialization leaves an existing file untouched. `arclint domain explain`
prints the same ArcLint meanings used by help, guided authoring, JSON
output, and the extension SDK. Declaring knowledge never creates a
Diagnostic by itself; enabled Rules under `arclint check` (such as
[Visible Domain Contracts](/docs/contracts/)) decide whether the model
is enforced.

### What the language buys you

Modules and Rules speak in Module names, and the recorded language is
where those names come from. Once the two agree, recording a term is
enough to extend the architecture:

- A structure Rule with `each: domain.aggregates` expands once per
  recorded aggregate. Record a new aggregate and the Rule now requires
  its home (`internal/domain/{name:flatcase}/root.go`) without an edit
  to `rules.yaml`.
- A Module named after a bounded context is that context in the
  dependency graph. The `respect-context-relations` Extension reads the
  recorded context map and judges imports between context-named
  Modules by the recorded relation, so recording a new context and its
  relation is what adds the import Rule.
- An `invariants` Rule requires every recorded invariant of a context
  to be visible in the owner's source, so recording an invariant states
  a Claim the next `arclint check` evaluates.

This repository runs the first two on itself (`domain-model/aggregate-skeleton`
and `domain-model/contexts-respect-relations` in
[rules.yaml](https://github.com/wixregiga/arclint/blob/main/rules.yaml));
the boxoffice proving ground under `testing/boxoffice` runs all three,
with `entities/contracts-visible` as the `invariants` Rule.

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
