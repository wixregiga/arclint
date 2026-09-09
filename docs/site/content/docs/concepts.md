+++
title = "Concepts"
description = "Zones, Rules, Patterns, Assurance, and how ArcLint reports findings."
weight = 2
+++

## Zones

A Zone is a named set of files, defined by path globs in `rules.arclint.yaml`.
Zones are the vocabulary other Rules use: consumes allow-lists,
layers, and protections refer to Zone names, never raw paths.

A Zone is logical, not a folder. One Zone may gather files from
many roots, and one glob may reach into every slice of a vertically
sliced tree: `internal/*/domain/**` is the domain layer of every
feature, wherever the feature lives.

```yaml
zones:
  entities:
    paths: "internal/*/domain/**"
    description: "Every slice's domain layer; depends on nothing."
  use_cases: "internal/*/app/**"
  transport: ["internal/*/http/**", "internal/*/grpc/**"]
  toolchain: ["Makefile", "go.mod"]
```

A glob matches files directly, and a glob naming a directory owns the
whole subtree. Overlapping Zones are legal: a file can belong to
several Zones, which makes umbrella Zones (`source: "internal/**"`)
cheap for repo-wide invariants.

Inspect the loaded map with `arclint context` or
`arclint context --zone <name>`.

## Import classes

Every import in every scanned file is classified before dependency
Rules run. The class names appear throughout the Rule surface:

| class | meaning |
|---|---|
| `internal` | resolves to a file inside this repository: another Zone, or undeclared internal code |
| `external` | a third-party dependency declared in your manifest: `go.mod` require, `package.json` dependencies, `pyproject.toml` |
| `stdlib` | the language's standard library (embedded tables generated from each toolchain) |
| `unknown` | none of the above; governed by `scan.unknown_imports: warn/error/ignore` |

So in an `imports` Rule, `internal: [app]` means "may import the app
Zone and nothing else internal", and `external: forbid` means "no
third-party libraries here at all". Go classification follows toolchain
semantics. TypeScript and Python are lexer-grade with documented
limits: computed specifiers like `import(x)` or
`importlib.import_module(name)` are invisible by design.

## Rules and constraints

`rules:` is one map keyed by Rule ID. Every Rule carries one Constraint:
the checkable proposition that must hold within its scope. The constraint
key selects its shape, and `on` names the Zones it judges. An optional
Rationale (`rationale`) records the author's reason for requiring it.
ArcLint derives the proposition from the Constraint and scope; it never
derives a Rationale when the author supplies none. Rule `description`
remains a deprecated alias for `rationale`, and using both is rejected.
Zone and Pattern descriptions keep their existing meanings.

- Zone-scoped: `imports` (what the Zone may depend on),
  `structure` (files it must or must not contain), `naming`,
  `content` (lines it must not contain), `invariants` (recorded domain
  contracts visible in source), and `uses` (an Extension).
- Graph-scoped, with the Zones in the constraint itself: `layers`,
  `imported_by` (who may import the one Zone under `on`),
  `independent`, and `acyclic`.

A Rule with two constraint keys is rejected: give each Constraint its own ID.
The [rule reference](/docs/rules/) lists every published Rule Type and
paste-ready YAML. `arclint rules` lists configured Rules;
`arclint rules <id>` shows one complete Rule when the selector matches
exactly.

## Patterns and adoption

A Pattern distributes Rules by reference. `extends` names it by exact
version and binds every Pattern Zone to local paths; the Pattern's
Rules load under the Pattern's namespace, and an entry under `rules:`
with no constraint is an Override of one of them: `severity`, `disable`
with a reason, `exclude`, or `suppress`. Nothing is copied.

A Pattern resolves offline first: from the binary that embeds it, then
from a vendored or authored copy under `.arclint/patterns`. A Registry
is read only when `install` or `vendor` asks for a Pattern that
resolves nowhere offline, and the copy it writes is verified against
its manifest on every load; a check never reaches the network. See
[Patterns](/docs/patterns/).

## Assurance

Every Rule Type states how strongly Enforcement can decide its Constraint.
Findings and Rule detail carry the label:

| Assurance | basis |
|---|---|
| `exact` | fully decides the Constraint within a documented analysis limit |
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
(`domain/stdlib-only`). A material Constraint change needs a new ID. Rules an
extended Pattern distributes carry the Pattern's namespace/name
(`arclint/vertical:domain/stdlib-only`), so the prefix selector
`arclint/vertical:` narrows to what that Pattern distributes and
`arclint/` to everything the namespace publishes. `arclint rules
<selector>` lists matches or
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
`domain.arclint.yaml` beside `rules.arclint.yaml`. It is first-class
project knowledge, not hidden ArcLint machinery. The file is organized
by bounded context, names as keys: each context holds its aggregates,
value objects, events, services, specifications, and open questions;
an aggregate holds its identity, its entities, its invariants, and its
assertions; a value object holds its invariants; top-level relations
name how contexts connect. ArcLint owns the meanings of the concepts
(`arclint domain explain` prints them with their sources); the project
supplies names, definitions, aliases, statements, and owners.

| concept | spelling | meaning |
|---|---|---|
| Bounded Context | `bounded_context` | A linguistic boundary; terms are defined inside one context, and its code is located from where its recorded terms are declared, narrowed to a matching Zone only when the ruleset declares one. |
| Aggregate | `aggregate` | A consistency boundary reached through its identity; the root enforces the invariants recorded under it, and its code is where the root is declared. |
| Entity | `entity` | A member of an aggregate whose identity matters as it changes over time. |
| Value Object | `value_object` | A domain value defined entirely by its attributes, with no identity of its own; built through one constructor when it records an invariant. |
| Invariant | `invariant` | What always holds inside its owner; keyed, since the root's enforcing method is `Ensure` followed by the key. |
| Assertion | `assertion` | What holds when one operation of the root completes (`on`); the root's checking method is `Assert` followed by the key. |
| Domain Event | `domain_event` | Something that has completed in the domain and that the project cares to record (file section: `events`). |
| Domain Service | `domain_service` | An operation of the model that belongs to no aggregate. |
| Specification | `specification` | A named predicate carrying a satisfaction method. |
| Question | `question` | What the project has not decided yet, recorded instead of guessed. |

The JSON Schema for the file is generated by the binary: `arclint domain
schema` prints it, `--write` puts a local copy at
`.arclint/schemas/domain.arclint.schema.json`, and arclint publishes it
as `docs/schemas/domain.arclint.schema.json`. Inspect and maintain the model with
`arclint domain`; start an empty model with `arclint domain init`.
Initialization leaves an existing file untouched. `arclint domain explain`
prints the same ArcLint meanings used by help, guided authoring, JSON
output, and the extension SDK. Declaring knowledge never creates a
Diagnostic by itself; the built-in rules under `arclint check` (see
[Domain Contracts](/docs/contracts/)) decide whether the model is
enforced, and they apply as soon as one context is recorded.

### What the language buys you

Zones and Rules speak in Zone names, and the recorded language is
where those names come from. Once the two agree, recording a term is
enough to extend the architecture:

- A structure Rule with `each: domain.aggregates` expands once per
  recorded aggregate. Record a new aggregate and the Rule now requires
  its home (`internal/domain/{name:flatcase}/root.go`) without an edit
  to `rules.arclint.yaml`.
- A Zone named after a bounded context is that context in the
  dependency graph. The built-in `bounded_context/isolated` and
  `context_relation/imports-follow-influence` rules judge imports
  between context-named Zones by the recorded relation, so recording
  a new context and its relation is what adds the import Rule.
- The built-in `invariant/enforced-at-every-mutation` rule requires
  every recorded invariant of an aggregate to be a method of the root
  that every constructor and command calls, so recording an invariant
  adds a Constraint the next `arclint check` evaluates.

The boxoffice proving ground under `testing/boxoffice` runs all three
(the first through its own `aggregate-slices` structure Rule, the other
two built in), with its honest remaining gaps in
`.arclint/baseline.v2.json`.

## Validation layers

`rules.arclint.yaml` passes three gates before anything runs: YAML syntax, the
published JSON Schema (the same file that powers editor completion and
`arclint rules schema`), and semantic validation (Zone references,
regex compilation, Extension parameter schemas). Extension Rule params
are validated against each Extension's declared schema before a line of
extension code executes.

## Exit codes

`0` is a clean command. `1` means the gate failed: an active
error-severity Violation or an error-severity operational Diagnostic
(or a Rule Test expectation mismatch). `2` is configuration or usage
error.
