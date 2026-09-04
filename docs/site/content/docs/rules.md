+++
title = "Rule reference"
description = "Every published Rule Type: its assertion key in rules.yaml, and a paste-ready example."
weight = 3
+++

The published Rule Types are a finite ArcLint-owned enum. Configure them
in `rules.yaml`; Extensions do not add new types. The same shapes power
`arclint rules schema`, editor completion, and `arclint rules <id>`
detail output.

## The shape of a Rule

`rules:` is a map keyed by Rule ID. Every entry carries exactly one
assertion key, and the assertion key decides the Rule Type:

| assertion key | Rule Type | judges |
|---|---|---|
| `imports` | consumes | the Modules under `on` |
| `structure` | structure | the Modules under `on` |
| `naming` | naming | the Modules under `on`, narrowed by `files` |
| `content` | content | the Modules under `on` or the repository, narrowed by `files` |
| `invariants` | invariants | the Modules under `on` |
| `uses` (+ `with`) | extension | the Modules under `on` or the repository, narrowed by `files` |
| `imported_by` | protected | the one Module under `on` |
| `layers` | layers | the repository (the key names its Modules) |
| `independent` | independence | the repository (the key names its folders) |
| `acyclic` | acyclic | the repository (the key names its Modules) |

The common keys beside the assertion:

| key | meaning |
|---|---|
| `description` | the Claim: the architectural proposition the Rule states, printed by `arclint rules`, `arclint context`, and `AGENTS.md` (derived from the assertion when absent) |
| `severity` | `error` (default), `warning`, or `info`; independent from Assurance |
| `on` | one Module name or a list; required, optional, or forbidden per the table above |
| `files` | one glob or a list narrowing the judged files; accepted by naming, content, and uses only |
| `exclude` | `{paths, modules, reason}`: files the Rule does not judge |
| `suppress` | `{paths, reason}`: files whose findings are kept but not active |

A Rule ID is `LOCAL` or `NAMESPACE:LOCAL`, where the local part is
`segment/segment` in lower-case kebab. Repository Rules use bare local
IDs (`domain/stdlib-only`); Rules an extended Pattern distributes carry
the Pattern's namespace (`arclint:domain/stdlib-only`). An entry with no
assertion key is an Override of a Pattern Rule; see
[Patterns](/docs/patterns/).

## modules

Name the parts of your repository; Rules refer to these names.

- where: top-level `modules:`

A Module is a named set of files selected by path globs. A glob matches
files directly, and a glob naming a directory owns the whole subtree.
A Module is logical: its globs may span many roots, and `*` in a
middle segment selects the same layer inside every slice of a
vertically sliced tree. Overlapping Modules are legal: a file may
belong to several at once. Three spellings are accepted: one glob, a
list of globs, or an object with `paths` and `description`. The
description is what `arclint context` and the generated `AGENTS.md` say
about the Module, so give real Modules one.

```yaml
modules:
  cmd: "cmd/**"
  toolchain: ["Makefile", "go.mod", ".golangci.yml"]
  entities:
    paths: "internal/*/domain/**"
    description: "Every slice's domain layer; depends on nothing."
  transport: ["internal/*/http/**", "internal/*/grpc/**"]
```

Inspect declared Modules with `arclint context` (repository scope) or
`arclint context --module <name>`.

## imports

What a Module may import: other Modules, third-party, stdlib.

- assertion key: `imports`
- Rule Type: `consumes`
- Assurance: `exact`
- `on`: required

`internal` names declared Modules this Module may import; absent means
unrestricted internal imports, and `[]` means no other declared Module.
The owning Module is always permitted implicitly. `external` and
`stdlib` are `allow` (default) or `forbid`.

```yaml
rules:
  entities/stdlib-only:
    description: "The entities layer imports no other Module and no third-party package."
    on: entities
    imports:
      internal: []
      external: forbid
```

## structure

Paths that must exist (`require`) or must not (`forbid`).

- assertion key: `structure`
- Rule Type: `structure`
- Assurance: `exact`
- `on`: required

At least one non-empty `require` or `forbid` list is required. Each
`require` glob must match at least one member file; no member file may
match a `forbid` glob.

```yaml
rules:
  composition/entrypoint:
    description: "Every binary has a main.go, and the legacy tree is gone."
    on: composition
    structure:
      require: ["cmd/**/main.go"]
      forbid: ["internal/legacy/**"]
```

`each` derives the globs from a recorded vocabulary collection in
`ubiquitous-language.yaml`, resolving `{name:<case>}` once per recorded
term. The published sources are `domain.aggregates`, `domain.entities`,
`domain.value_objects`, `domain.events`, `domain.contexts`,
`domain.invariants`, `domain.assertions`, and `domain.specifications`;
the cases are `flatcase`, `snake_case`, `kebab-case`, `camelCase`, and
`PascalCase`. When nothing is recorded the Rule holds vacuously and its
Claim says so.

```yaml
rules:
  entities/aggregate-slices:
    description: "Every recorded aggregate owns a slice with its file, its repository interface, and its tests."
    on: entities
    structure:
      each: domain.aggregates
      require:
        - "internal/entities/{name:flatcase}/{name:flatcase}.go"
        - "internal/entities/{name:flatcase}/repository.go"
```

## naming

File names follow a case convention or regex.

- assertion key: `naming`
- Rule Type: `naming`
- Assurance: `exact`
- `on`: required; `files` accepted

Applies to the file stem (base name minus the final extension). Cases:
`kebab-case`, `snake_case`, `camelCase`, `PascalCase`, or
`regex:<pattern>`; combine alternatives with `|`. The assertion is the
case spec itself, or `{case: ...}`.

```yaml
rules:
  source/snake-case:
    description: "Go file names use snake_case."
    on: source
    files: "internal/**/*.go"
    naming: snake_case
```

## content

No line of a selected file matches a regular expression.

- assertion key: `content`
- Rule Type: `content`
- Assurance: `exact`
- `on`: optional (absent means the whole repository); `files` accepted

`forbid` is an RE2 pattern matched against every line of every
selected file. A match is a line-anchored Violation. This covers the
technology fences that used to need an Extension: no `panic`, no
`net/http` in the domain, one logging voice.

```yaml
rules:
  domain/no-panic:
    description: "Domain code never panics; a representation that cannot become a value is an error."
    on: domain
    files: "internal/domain/**/*.go"
    content:
      forbid: '\bpanic\('

  source/slog-only:
    description: "The server logs through slog only."
    on: source
    files: "internal/**/*.go"
    content:
      forbid: '\bfmt\.Print|\blog\.(Print|Fatal|Panic)'
```

## invariants

Recorded domain contracts are visible in source.

- assertion key: `invariants`
- Rule Type: `invariants`
- Assurance: `exact`
- `on`: required

Every cluster invariant, assertion, and specification recorded in
`ubiquitous-language.yaml` for the owners that live in the Module must
exist as a named method called from its join points. `{}` takes the
default posture; `closed: true` additionally requires every exported
error-returning function in the owner's files to call the cluster
method. See [Domain Contracts](/docs/contracts/).

```yaml
rules:
  entities/contracts-visible:
    description: "Every aggregate's invariants are visible through its cluster method."
    on: entities
    invariants: {}
```

## uses

Delegate enforcement to a named Extension.

- assertion key: `uses` with optional `with`
- Rule Type: `extension`
- Assurance: `heuristic`
- `on`: optional (absent means the whole repository); `files` accepted

`uses` is the extension rule name registered under
`.arclint/extensions` or supplied by an extended Pattern. `with` holds
parameters validated host-side against the extension's published
schema before any extension code runs. See
[TypeScript extensions](/docs/extensions/).

```yaml
rules:
  entities/aggregates-encapsulate:
    description: "The struct of every recorded aggregate has no exported fields."
    on: entities
    files: "internal/entities/**/*.go"
    uses: aggregate-encapsulation
    with:
      root: "internal/entities"
```

## imported_by

Who may import one Module.

- assertion key: `imported_by`
- Rule Type: `protected`
- Assurance: `exact`
- `on`: required, exactly one Module

Protection is checked from the importer side. The Module under `on`
may import itself; each Module in the list may also import it. A file
is an allowed importer when any of its Modules is in that set. `[]`
means nothing else imports it.

```yaml
rules:
  infrastructure/composition-only:
    description: "Only composition imports infrastructure."
    on: infrastructure
    imported_by: [composition]
```

## layers

An ordered stack: a Module imports only same or lower layers.

- assertion key: `layers`
- Rule Type: `layers`
- Assurance: `exact`
- `on`: not accepted

Orders Modules highest first. A Module may import its own layer or
lower layers, never a higher one. At least two Modules, no duplicates.

```yaml
rules:
  dependencies/application-inward:
    description: "Dependencies point inward: application, then domain."
    layers: [application, domain]
```

## independent

Sibling folders selected by globs may not import each other.

- assertion key: `independent`
- Rule Type: `independence`
- Assurance: `exact`
- `on`: not accepted

Each glob selects member folders from observed files. A candidate is
dropped when a declared Module owns that folder's subtree.

```yaml
rules:
  features/independent:
    description: "Feature folders never import each other."
    independent: ["internal/*"]
```

## acyclic

No import cycles among the named Modules.

- assertion key: `acyclic`
- Rule Type: `acyclic`
- Assurance: `exact`
- `on`: not accepted

A list names the Modules to check (at least two); `{}` covers every
declared Module.

```yaml
rules:
  dependencies/acyclic:
    description: "Dependencies among the layer Modules contain no cycle."
    acyclic: [composition, delivery, infrastructure, application, domain]
```

## exclude and suppress

Both keys sit beside any assertion, and both are the whole content of
an Override. An Exclusion removes files from what the Rule judges; a
Suppression keeps the finding in the Assessment and in JSON output but
takes it out of the active count. Each carries a `reason` so the
decision stays inspectable in `arclint rules <id>`.

```yaml
rules:
  source/snake-case:
    description: "Go file names use snake_case."
    on: source
    files: "internal/**/*.go"
    naming: snake_case
    exclude:
      paths: ["internal/generated/**"]
      reason: "generated code keeps the generator's names"
    suppress:
      paths: ["internal/legacy/OldHandler.go"]
      reason: "renamed in the next release; tracked in AL-40"
```

## scan

Walk policy: excludes, testdata, unknown-import severity.

- where: top-level `scan:`

Tunes the tree walk and classification policy. `exclude` adds glob
patterns to the built-in skip set (`.git`, `.hg`, `.svn`, `.arclint`,
`vendor`, `node_modules`, and `testdata` unless included). The walker
never follows symlinks. `include_testdata: true` observes `testdata`
directories. `unknown_imports` sets what happens when an import
classifies neither internal, external, nor stdlib: `warn` (default),
`error`, or `ignore`.

```yaml
scan:
  exclude: ["gen/**"]
  include_testdata: false
  unknown_imports: error
```

## runtime

Which language targets produce Language Facts.

- where: top-level `runtime:`

A list of `go`, `ts`, and/or `py`. `arclint init --languages` drafts
this block from the languages you select.

```yaml
runtime: [go, ts]
```

## extends

Adopt a Pattern's Rules by reference, binding its Modules to paths.

- where: top-level `extends:`

Each entry names one Pattern by exact reference and binds every Module
the Pattern lists to the paths it owns here. The Pattern's Rules then
load under their namespaced IDs, and Overrides under those IDs adjust
severity, disable with a reason, exclude, or suppress. The complete
adoption grammar lives in [Patterns](/docs/patterns/).

```yaml
extends:
  - pattern: arclint/vertical@0.1.0
    bind:
      domain: "internal/*/domain/**"
      application: "internal/*/application/**"
      infra: "internal/*/infra/**"
      app: "internal/app/**"
      shared: "internal/shared/**"
      composition: "cmd/**"
```
