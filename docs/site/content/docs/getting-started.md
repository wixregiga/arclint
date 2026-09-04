+++
title = "Getting started"
description = "Install, init, check: contracts enforced in two minutes."
weight = 1
+++

## Install

arclint is one static binary (CGO disabled). Build it from source with
Go:

```bash
git clone https://github.com/wixregiga/arclint && cd arclint
make build   # produces ./arclint
```

Or run it as a container. The image is built from `scratch` (the binary
is self-contained; the TypeScript extension runtime is in-process), and
the default command checks the mounted repository:

```bash
make docker
docker run --rm -v "$PWD":/repo "arclint:$(cat cmd/arclint/VERSION)"
```

The CLI binary and image tag both use `cmd/arclint/VERSION`.

## Initialize a repository

```bash
cd ~/projects/your-repo
arclint init
```

`init` drafts a `rules.arclint.yaml` from explicit language choices. It does not
scan the tree. The no-flag default is `--pattern bare`: a commented
single-module draft. Default languages is `go`. Pass others with
`--languages`:

```bash
arclint init --languages go,ts
```

Adopt a Pattern instead of starting bare:

```bash
arclint init --pattern arclint/vertical@0.1.0   # or --pattern vertical
```

That writes a `rules.arclint.yaml` whose `extends` block pins the Pattern by
exact reference and binds each Pattern Module to the paths the Pattern
suggests; edit the bindings to match your tree. Nothing is copied into
`.arclint/extensions`: the Pattern's Rules and Extensions load by
reference on every run. `--pattern bare` writes only the draft ruleset.
An existing `rules.arclint.yaml` is refused unless you pass `--force`.

A repository that already has a `rules.arclint.yaml` adds a Pattern with
`install` instead, which extends the file in place and keeps its
comments:

```bash
arclint patterns                      # what resolves offline
arclint patterns install domain-model # extend rules.arclint.yaml with it
arclint patterns install acme/layers  # fetched from the Registry, vendored, then extended
```

A Pattern the binary does not embed is vendored under
`.arclint/patterns/<namespace>/<name>/` with a manifest before the
ruleset extends it, so `arclint check` never touches the network. See
[Patterns](/docs/patterns/) for vendoring, Registries, and publishing
your own.

Accepted values for `--languages` are `go`, `ts`, and `py`
(comma-separated).

On success:

```
wrote /path/to/rules.arclint.yaml
next: declare your modules, then run `arclint check .`
```

The bare draft sets `runtime`, one Module `source` covering `**`, and
one vacuously satisfied `imports` Rule so the file loads and `check`
stays clean until you split Modules:

```yaml
# ArcLint architecture contracts.
# Grow this file module by module: declare real Modules under `modules`,
# then state what each may import under `rules`.
# Query commands: arclint rules [selector] · arclint context <path>

runtime: [go]

scan:
  # error | warn | ignore for imports that classify neither stdlib,
  # internal, nor declared in the dependency manifest.
  unknown_imports: warn

modules:
  # A Module is a name and the paths it owns: one glob, a list of globs,
  # or {paths, description}. Split into real Modules as the architecture
  # takes shape.
  source: "**"

rules:
  # Every Rule has an id, the Module(s) it judges under `on`, and exactly
  # one assertion: imports, structure, naming, content, layers,
  # imported_by, independent, acyclic, invariants, or uses.
  source/dependencies:
    description: "Source imports no other declared Module."
    on: source
    imports:
      # An allow-list of other declared Modules. Empty means this Module
      # may import no other declared Module; with one Module this is
      # vacuously true and starts binding the moment you split Modules.
      internal: []
```

A grown file reads the same way, one Rule per architectural claim:

```yaml
runtime: [go]

modules:
  domain:
    paths: "internal/domain/**"
    description: "Aggregates and domain values; stdlib-only."
  application:
    paths: "internal/application/**"
    description: "Use cases coordinating domain objects through ports."
  infrastructure: "internal/infrastructure/**"
  composition: "cmd/**"

rules:
  domain/stdlib-only:
    description: "The domain imports no other Module and no third-party package."
    on: domain
    imports:
      internal: []
      external: forbid

  domain/no-panic:
    description: "Domain code never panics."
    on: domain
    files: "internal/domain/**/*.go"
    content:
      forbid: '\bpanic\('

  application/through-ports:
    description: "Use cases import only the domain."
    on: application
    imports:
      internal: [domain]
      external: forbid

  infrastructure/composition-only:
    description: "Only composition imports infrastructure."
    on: infrastructure
    imported_by: [composition]

  dependencies/acyclic:
    description: "Module dependencies contain no cycle."
    acyclic: {}
```

When you add TypeScript Extensions later, `arclint sdk init` writes
`arclint.d.ts` and `tsconfig.json` under `.arclint/extensions`.

## Check

```bash
arclint check .
arclint check . --format json   # Diagnostic array for CI
```

Human output lists active Violations and operational or coverage
Diagnostics, then a summary line:

```
0 active finding(s) · 0 suppressed · 0 baselined · 1 rule(s) applied
```

Exit codes: `0` clean, `1` when the gate fails (an active error-severity
Violation or an error-severity operational Diagnostic), `2`
configuration or usage error.

Optional flags: `--only` / `--exclude` (Rule ids, prefixes, or patterns)
and `--no-baseline` to evaluate without subtracting the committed
baseline.

## Adopt an existing repository

A repo with history usually has Violations on day one. Capture them
instead of fixing everything first:

```bash
arclint baseline capture   # writes .arclint/baseline.v2.json
arclint check .
```

`baseline capture` reports how many findings it adopted, for example
`baseline captured: 1 finding(s) across 1 applied rule(s)`. After
capture, a clean human summary looks like
`0 active finding(s) · 0 suppressed · 1 baselined · 1 rule(s) applied`.

Commit the baseline file. From then on only new findings fail the run;
baselined findings stay counted. When adopted findings disappear, check
prints a coverage note so you can shrink the snapshot:

```
coverage: baseline: 1 adopted finding(s) no longer occur; refresh the baseline
```

```bash
arclint baseline refresh   # drops stale entries after comparison
```

## Adjust the modules

Module boundaries are yours. Edit the `modules:` block in `rules.arclint.yaml`,
then inspect what binds where:

```bash
arclint context                  # repository scope
arclint context path/to/file.go  # everything binding a path
arclint context --module source  # one declared Module
```

`context` reports scope, languages, configured Rule count, each Module's
paths and import allow-list, and the Rules that apply. Re-run
`arclint check .` when the Modules look right.

## Inspect Rules and schema

```bash
arclint rules                          # one line per configured Rule
arclint rules source/dependencies      # full detail for one Rule
arclint rules schema                   # JSON Schema for rules.arclint.yaml
```

For editor completion, write a local copy of the schema and point a YAML
language server at it:

```bash
arclint rules schema --write           # .arclint/schemas/rules.arclint.schema.json
```

```yaml
# yaml-language-server: $schema=.arclint/schemas/rules.arclint.schema.json
```

`arclint init` writes that modeline when the local copy exists;
otherwise it names the published copy, kept identical to
`arclint rules schema` by a drift test:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/wixregiga/arclint/main/docs/schemas/rules.arclint.schema.json
```

`arclint domain schema --write` does the same for `domain.arclint.yaml`.
