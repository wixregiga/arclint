+++
title = "Rule reference"
description = "Every published Rule Type: where it goes in rules.yaml, and a paste-ready example."
weight = 3
+++

The published Rule Types are a finite ArcLint-owned enum. Configure them
in `rules.yaml`; Extensions do not add new types. The same shapes power
`arclint rules schema`, editor completion, and `arclint rules <id>`
detail output.

Every Rule needs an explicit `id`. Severity is optional on each Rule
(`error` default; also `warning`, `info`) and is independent from
Assurance.

## modules

Name the parts of your repository; other Rules refer to these names.

- where: top-level `modules:`

A Module is a named set of files selected by path globs. A glob matches
files directly, and a glob naming a directory owns the whole subtree.
Overlapping Modules are legal: a file may belong to several at once.

```yaml
modules:
  cmd:
    paths: ["cmd/**"]
  entities:
    paths: ["internal/entities/**"]
    description: "Domain types and invariants; depends on nothing."
```

Inspect declared Modules with `arclint context` (repository scope) or
`arclint context --module <name>`.

## consumes

What a Module may import: other Modules, third-party, stdlib.

- where: `contracts.<module>.consumes`
- Rule Type: `consumes`
- Assurance: `exact`

At least one restriction is required: an `internal` allow-list,
`external: forbid`, or `stdlib: forbid`. `internal` names declared
Modules this Module may import; absent means unrestricted internal
imports, and `[]` means no other declared Module. The owning Module is
always permitted implicitly. `external` and `stdlib` default to
`allow`.

```yaml
contracts:
  entities:
    consumes:
      id: "app:entities/stdlib-only"
      internal: []
      external: forbid
      stdlib: allow
```

## structure

Paths that must exist (`require`) or must not (`forbid`).

- where: `contracts.<module>.invariants`
- Rule Type: `structure` (`kind: structure`)
- Assurance: `exact`

At least one non-empty `require` or `forbid` list is required. Each
`require` glob must match at least one member file; no member file may
match a `forbid` glob.

```yaml
contracts:
  repo:
    invariants:
      - id: "app:repo/entrypoint"
        kind: structure
        require: ["cmd/**/main.go"]
        forbid: ["internal/legacy/**"]
```

## naming

File names follow a case convention or regex.

- where: `contracts.<module>.invariants`
- Rule Type: `naming` (`kind: naming`)
- Assurance: `exact`

Applies to the file stem (base name minus the final extension). Cases:
`kebab-case`, `snake_case`, `camelCase`, `PascalCase`, or
`regex:<pattern>`; combine alternatives with `|`. `files` narrows the
Rule to a glob inside the Module.

```yaml
contracts:
  src:
    invariants:
      - id: "app:src/snake-case"
        kind: naming
        files: "internal/**/*.go"
        case: snake_case
```

## extension

Delegate enforcement to a named Extension under `.arclint/extensions`.

- where: `contracts.<module>.invariants`
- Rule Type: `extension` (`kind: extension`)
- Assurance: `heuristic`

`uses` is the extension rule name. `with` holds parameters validated
host-side against the extension's published schema before any extension
code runs. `files` optionally narrows member files. See
[TypeScript extensions](/docs/extensions/).

```yaml
contracts:
  domain:
    invariants:
      - id: "app:domain/no-panic"
        kind: extension
        files: "internal/domain/**/*.go"
        uses: forbid-content
        with:
          pattern: '\bpanic\('
```

## layers

An ordered stack: a Module imports only same or lower layers.

- where: top-level `dependencies:`
- Rule Type: `layers` (`kind: layers`)
- Assurance: `exact`

Orders Modules highest first. A Module may import its own layer or
lower layers, never a higher one. At least two Modules, no duplicates.

```yaml
dependencies:
  - id: "app:deps/layers"
    kind: layers
    layers: [application, domain]
```

## protected

A protected Module is importable only by itself and an allow-listed set
of additional Modules.

- where: top-level `dependencies:`
- Rule Type: `protected` (`kind: protected`)
- Assurance: `exact`

Protection is checked from the importer side. The protected Module may
import itself; each Module in `allow` may also import it. A file is an
allowed importer when any of its Modules is in that set.

```yaml
dependencies:
  - id: "app:deps/infra-protected"
    kind: protected
    module: infrastructure
    allow: [composition]
```

## independence

Sibling Folders selected by globs may not import each other.

- where: top-level `dependencies:`
- Rule Type: `independence` (`kind: independence`)
- Assurance: `exact`

Each glob in `folders` selects member folders from observed files. A
candidate is dropped when a declared Module owns that folder's subtree.

```yaml
dependencies:
  - id: "vertical:features/independent"
    kind: independence
    folders: ["internal/*"]
```

## acyclic

No import cycles among the named Modules.

- where: top-level `dependencies:`
- Rule Type: `acyclic` (`kind: acyclic`)
- Assurance: `exact`

An absent `modules` list covers every declared Module.

```yaml
dependencies:
  - id: "app:deps/acyclic"
    kind: acyclic
    modules: [composition, delivery, infrastructure, application, domain]
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
